#!/usr/bin/env bash
# autopull universal installer/updater
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Joel-Claw/autopull/main/install.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/Joel-Claw/autopull/main/install.sh | bash -s -- --uninstall
#
# Options:
#   --uninstall     Remove autopull
#   --version TAG   Install specific version (default: latest)
set -euo pipefail

REPO="Joel-Claw/autopull"
BINARY="/usr/local/bin/autopull"
SERVICE_FILE="/etc/systemd/system/autopull.service"
VERSION="${1:-latest}"
UNINSTALL=false

# Parse args
for arg in "$@"; do
    case "$arg" in
        --uninstall) UNINSTALL=true ;;
        --version=*) VERSION="${arg#--version=}" ;;
        --version)   shift; VERSION="${1:-latest}" ;;
    esac
done

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[autopull]${NC} $*"; }
warn()  { echo -e "${YELLOW}[autopull]${NC} $*"; }
error() { echo -e "${RED}[autopull]${NC} $*" >&2; exit 1; }

# ── Uninstall ──────────────────────────────────────────────────────────────

if [ "$UNINSTALL" = true ]; then
    echo "Uninstalling autopull..."
    systemctl stop autopull 2>/dev/null || true
    systemctl disable autopull 2>/dev/null || true
    rm -f "$BINARY" "$SERVICE_FILE"
    systemctl daemon-reload 2>/dev/null || true
    info "Uninstalled autopull"
    exit 0
fi

# ── Check root ─────────────────────────────────────────────────────────────

if [ "$(id -u)" -ne 0 ]; then
    error "run as root: curl ... | sudo bash"
fi

# ── Detect distro ──────────────────────────────────────────────────────────

detect_distro() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        echo "${ID:-unknown}"
    elif command -v pacman &>/dev/null; then
        echo "arch"
    elif command -v apt-get &>/dev/null; then
        echo "debian"
    elif [ -d /nix ]; then
        echo "nixos"
    else
        echo "unknown"
    fi
}

DISTRO=$(detect_distro)
info "Detected distro: $DISTRO"

# ── Install dependencies ───────────────────────────────────────────────────

install_deps() {
    case "$DISTRO" in
        debian|ubuntu|pop|linuxmint|elementary)
            apt-get update -qq
            apt-get install -y -qq git golang-go
            ;;
        arch|manjaro|endeavouros|garuda)
            pacman -S --noconfirm --needed git go
            ;;
        fedora|rhel|centos|rocky|alma)
            dnf install -y git golang
            ;;
        opensuse*|sles)
            zypper install -y git go
            ;;
        nixos)
            if ! command -v go &>/dev/null; then
                nix-env -iA nixpkgs.go nixpkgs.git
            fi
            ;;
        alpine)
            apk add git go
            ;;
        *)
            warn "Unknown distro, assuming Go and git are installed"
            command -v go &>/dev/null || error "Go not found. Install it manually: https://go.dev/dl/"
            command -v git &>/dev/null || error "git not found. Install it manually."
            ;;
    esac
}

# ── Check if already installed (update mode) ───────────────────────────────

INSTALLED_VERSION=""
if command -v autopull &>/dev/null; then
    INSTALLED_VERSION=$(autopull --version 2>/dev/null || echo "unknown")
    info "autopull already installed ($INSTALLED_VERSION), updating..."
fi

# ── Install deps ───────────────────────────────────────────────────────────

command -v git &>/dev/null || install_deps
command -v go &>/dev/null || { install_deps; }

command -v git &>/dev/null || error "git not found after package install"
command -v go &>/dev/null || error "Go not found after package install"

# ── Download and build ─────────────────────────────────────────────────────

TMPDIR=$(mktemp -d)
cleanup() { rm -rf "$TMPDIR"; }
trap cleanup EXIT

if [ "$VERSION" = "latest" ]; then
    CLONE_URL="https://github.com/${REPO}.git"
    CLONE_BRANCH="main"
else
    CLONE_URL="https://github.com/${REPO}.git"
    CLONE_BRANCH="$VERSION"
fi

info "Downloading autopull (${VERSION})..."
git clone --depth 1 --branch "$CLONE_BRANCH" "$CLONE_URL" "$TMPDIR/autopull" 2>/dev/null || {
    # Branch might be a tag
    git clone --depth 1 "$CLONE_URL" "$TMPDIR/autopull" 2>/dev/null && \
    cd "$TMPDIR/autopull" && git checkout "$VERSION" 2>/dev/null
}

cd "$TMPDIR/autopull"

info "Building..."
go build -ldflags="-s -w" -o autopull ./cmd/autopull

# ── Install binary ──────────────────────────────────────────────────────────

install -m 755 autopull "$BINARY"
info "Installed: $BINARY"

# ── Install systemd service ─────────────────────────────────────────────────

if [ -d /etc/systemd/system ]; then
    # Preserve existing User/Group if service already exists
    EXISTING_USER=""
    if [ -f "$SERVICE_FILE" ]; then
        EXISTING_USER=$(grep -oP '^User=\K.*' "$SERVICE_FILE" 2>/dev/null || true)
        EXISTING_GROUP=$(grep -oP '^Group=\K.*' "$SERVICE_FILE" 2>/dev/null || true)
    fi

    cat > "$SERVICE_FILE" <<'EOF'
[Unit]
Description=AutoPull - Automatic Git Pull Daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/autopull daemon
Restart=on-failure
RestartSec=30

[Install]
WantedBy=multi-user.target
EOF

    # Set user from SUDO_USER or preserve existing
    SVC_USER="${SUDO_USER:-$EXISTING_USER}"
    SVC_GROUP="${SUDO_USER:-$EXISTING_GROUP}"
    if [ -n "$SVC_USER" ]; then
        sed -i "s/\[Service\]/[Service]\nUser=$SVC_USER\nGroup=$SVC_GROUP/" "$SERVICE_FILE"
    fi

    systemctl daemon-reload
    info "Installed: $SERVICE_FILE"

    # Enable if not already enabled
    if ! systemctl is-enabled autopull &>/dev/null; then
        systemctl enable autopull
        info "Service enabled (not started). Add repos first, then: systemctl start autopull"
    else
        # Restart if it was already running (update)
        systemctl is-active autopull &>/dev/null && systemctl restart autopull
        info "Service restarted"
    fi
fi

# ── Done ────────────────────────────────────────────────────────────────────

echo ""
info "Done! $(autopull 2>&1 | head -1 || true)"
echo ""
echo "Quick start:"
echo "  autopull add /path/to/repo"
echo "  autopull add /path/to/repo --interval 15m"
echo "  autopull add /path/to/repo --at \"09:00,18:00\""
echo "  sudo systemctl start autopull"
echo ""
echo "Update anytime:"
echo "  curl -fsSL https://raw.githubusercontent.com/Joel-Claw/autopull/main/install.sh | sudo bash"
echo ""
echo "Uninstall:"
echo "  curl -fsSL https://raw.githubusercontent.com/Joel-Claw/autopull/main/install.sh | sudo bash -s -- --uninstall"