#!/usr/bin/env bash
# pulley universal installer/updater
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Joel-Claw/pulley/main/install.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/Joel-Claw/pulley/main/install.sh | sudo bash uninstall
#
# Options:
#   --uninstall     Remove pulley
#   --version TAG   Install specific version (default: latest)
set -euo pipefail

REPO="Joel-Claw/pulley"
BINARY="/usr/local/bin/pulley"
SERVICE_FILE="/etc/systemd/system/pulley.service"
VERSION="latest"
UNINSTALL=false

# Parse args
for arg in "$@"; do
    case "$arg" in
        uninstall) UNINSTALL=true ;;
        --uninstall) UNINSTALL=true ;;
        --version=*) VERSION="${arg#--version=}" ;;
        --version)   shift; VERSION="${1:-latest}" ;;
        v=*) VERSION="${arg#v=}" ;;
    esac
done

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[pulley]${NC} $*"; }
warn()  { echo -e "${YELLOW}[pulley]${NC} $*"; }
error() { echo -e "${RED}[pulley]${NC} $*" >&2; exit 1; }

# ── Uninstall ──────────────────────────────────────────────────────────────

if [ "$UNINSTALL" = true ]; then
    echo "Uninstalling pulley..."
    systemctl stop pulley 2>/dev/null || true
    systemctl disable pulley 2>/dev/null || true
    rm -f "$BINARY" "$SERVICE_FILE"
    systemctl daemon-reload 2>/dev/null || true
    info "Uninstalled pulley"
    exit 0
fi

# ── Check root ─────────────────────────────────────────────────────────────

if [ "$(id -u)" -ne 0 ]; then
    error "run as root: curl ... | sudo bash"
fi

# ── Check git ──────────────────────────────────────────────────────────────

if ! command -v git &>/dev/null; then
    error "git is required but not installed. Install it first, then re-run."
fi

# ── Try binary install first ───────────────────────────────────────────────

ARCH=$(uname -m)
OS=$(uname -s)

CASE_OS=""; CASE_ARCH=""
case "$OS" in
    Linux)  CASE_OS="linux" ;;
    Darwin) CASE_OS="darwin" ;;
esac
case "$ARCH" in
    x86_64|amd64)  CASE_ARCH="amd64" ;;
    aarch64|arm64) CASE_ARCH="arm64" ;;
esac

if [ -n "$CASE_OS" ] && [ -n "$CASE_ARCH" ]; then
    # Get latest version
    LATEST_VERSION=""
    if command -v gh &>/dev/null; then
        LATEST_VERSION=$(gh release view --repo "$REPO" --json tagName --jq '.tagName' 2>/dev/null || echo "")
    fi
    if [ -z "$LATEST_VERSION" ]; then
        LATEST_VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null | grep '"tag_name"' | head -1 | sed -E 's/.*"tag_name":\s*"([^"]+)".*/\1/' || echo "")
    fi

    if [ -n "$LATEST_VERSION" ]; then
        BINARY_NAME="pulley-${LATEST_VERSION#v}-$CASE_OS-$CASE_ARCH"
        DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_VERSION/$BINARY_NAME"

        info "Trying pre-built binary for $CASE_OS/$CASE_ARCH..."
        TMPDIR=$(mktemp -d)
        cleanup() { rm -rf "$TMPDIR"; }
        trap cleanup EXIT

        if curl -fsSL "$DOWNLOAD_URL" -o "$TMPDIR/pulley" 2>/dev/null; then
            # Verify checksum if available
            CHECKSUMS_URL="https://github.com/$REPO/releases/download/$LATEST_VERSION/checksums-sha256.txt"
            if curl -fsSL "$CHECKSUMS_URL" -o "$TMPDIR/checksums.txt" 2>/dev/null; then
                EXPECTED=$(grep "$BINARY_NAME" "$TMPDIR/checksums.txt" | awk '{print $1}')
                if [ -n "$EXPECTED" ]; then
                    ACTUAL=$(sha256sum "$TMPDIR/pulley" | awk '{print $1}')
                    if [ "$EXPECTED" != "$ACTUAL" ]; then
                        warn "Checksum mismatch, falling back to source build"
                        rm -rf "$TMPDIR"
                        TMPDIR=$(mktemp -d)
                    else
                        info "Checksum verified ✓"
                    fi
                fi
            fi

            if [ -f "$TMPDIR/pulley" ] && [ -s "$TMPDIR/pulley" ]; then
                chmod +x "$TMPDIR/pulley"
                install -m 755 "$TMPDIR/pulley" "$BINARY"
                info "Installed binary: $BINARY"

                # Install systemd service
                if [ -d /etc/systemd/system ]; then
                    EXISTING_USER=""
                    if [ -f "$SERVICE_FILE" ]; then
                        EXISTING_USER=$(grep -oP '^User=\K.*' "$SERVICE_FILE" 2>/dev/null || true)
                        EXISTING_GROUP=$(grep -oP '^Group=\K.*' "$SERVICE_FILE" 2>/dev/null || true)
                    fi

                    cat > "$SERVICE_FILE" <<'SVCEOF'
[Unit]
Description=Pulley - Automatic Git Pull Daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/pulley daemon
Restart=on-failure
RestartSec=30

[Install]
WantedBy=multi-user.target
SVCEOF

                    SVC_USER="${SUDO_USER:-$EXISTING_USER}"
                    SVC_GROUP="${SUDO_USER:-$EXISTING_GROUP}"
                    if [ -n "$SVC_USER" ]; then
                        sed -i "s/\[Service\]/[Service]\nUser=$SVC_USER\nGroup=$SVC_GROUP/" "$SERVICE_FILE"
                    fi

                    systemctl daemon-reload
                    info "Installed: $SERVICE_FILE"

                    if ! systemctl is-enabled pulley &>/dev/null; then
                        systemctl enable pulley
                        info "Service enabled (not started yet). Add repos first, then: systemctl start pulley"
                    else
                        systemctl is-active pulley &>/dev/null && systemctl restart pulley
                        info "Service restarted"
                    fi
                fi

                INSTALLED_VERSION=$("$BINARY" version 2>&1 | head -1 || true)
                echo ""
                info "Done! $INSTALLED_VERSION (binary install)"
                echo ""
                echo "Quick start:"
                echo "  pulley add /path/to/repo"
                echo "  pulley add /path/to/repo --interval 15m --branches all"
                echo "  sudo systemctl start pulley"
                exit 0
            fi
        else
            info "No pre-built binary available, building from source..."
        fi
    fi
fi

# ── Fallback: build from source ────────────────────────────────────────────

info "Building from source..."

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
if command -v pulley &>/dev/null; then
    INSTALLED_VERSION=$(pulley --version 2>/dev/null || echo "unknown")
    info "pulley already installed ($INSTALLED_VERSION), updating..."
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

info "Downloading pulley (${VERSION})..."
git clone --depth 1 --branch "$CLONE_BRANCH" "$CLONE_URL" "$TMPDIR/pulley" 2>/dev/null || {
    # Branch might be a tag
    git clone --depth 1 "$CLONE_URL" "$TMPDIR/pulley" 2>/dev/null && \
    cd "$TMPDIR/pulley" && git checkout "$VERSION" 2>/dev/null
}

cd "$TMPDIR/pulley"

info "Building..."
go build -ldflags="-s -w" -o pulley ./cmd/pulley

# ── Install binary ──────────────────────────────────────────────────────────

install -m 755 pulley "$BINARY"
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
Description=Pulley - Automatic Git Pull Daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/pulley daemon
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
    if ! systemctl is-enabled pulley &>/dev/null; then
        systemctl enable pulley
        info "Service enabled (not started). Add repos first, then: systemctl start pulley"
    else
        # Restart if it was already running (update)
        systemctl is-active pulley &>/dev/null && systemctl restart pulley
        info "Service restarted"
    fi
fi

# ── Done ────────────────────────────────────────────────────────────────────

echo ""
info "Done! $(pulley 2>&1 | head -1 || true)"
echo ""
echo "Quick start:"
echo "  pulley add /path/to/repo"
echo "  pulley add /path/to/repo --interval 15m"
echo "  pulley add /path/to/repo --at \"09:00,18:00\""
echo "  sudo systemctl start pulley"
echo ""
echo "Update anytime:"
echo "  curl -fsSL https://raw.githubusercontent.com/Joel-Claw/pulley/main/install.sh | sudo bash"
echo ""
echo "Uninstall:"
echo "  curl -fsSL https://raw.githubusercontent.com/Joel-Claw/pulley/main/install.sh | sudo bash uninstall"