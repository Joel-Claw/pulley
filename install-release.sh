#!/usr/bin/env bash
# pulley release installer
# Usage:
#   curl -fsSL https://github.com/Joel-Claw/pulley/releases/latest/download/install.sh | sudo bash
#   Or download this script and run: sudo ./install.sh
set -euo pipefail

REPO="Joel-Claw/pulley"
BINARY="/usr/local/bin/pulley"
SERVICE_FILE="/etc/systemd/system/pulley.service"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[pulley]${NC} $*"; }
warn()  { echo -e "${YELLOW}[pulley]${NC} $*"; }
error() { echo -e "${RED}[pulley]${NC} $*" >&2; exit 1; }

# ── Check root ─────────────────────────────────────────────────────────────

if [ "$(id -u)" -ne 0 ]; then
    error "run as root: curl ... | sudo bash"
fi

# ── Check git ──────────────────────────────────────────────────────────────

if ! command -v git &>/dev/null; then
    error "git is required but not installed. Install it first, then re-run."
fi

# ── Detect arch ─────────────────────────────────────────────────────────────

ARCH=$(uname -m)
OS=$(uname -s)

case "$OS" in
    Linux)  OS_NAME="linux" ;;
    Darwin) OS_NAME="darwin" ;;
    *)      error "unsupported OS: $OS" ;;
esac

case "$ARCH" in
    x86_64|amd64)   ARCH_NAME="amd64" ;;
    aarch64|arm64)  ARCH_NAME="arm64" ;;
    *)              error "unsupported architecture: $ARCH" ;;
esac

# ── Get latest version ──────────────────────────────────────────────────────

info "Detecting latest release..."
if command -v gh &>/dev/null; then
    VERSION=$(gh release view --repo "$REPO" --json tagName --jq '.tagName' 2>/dev/null || echo "")
fi

if [ -z "$VERSION" ]; then
    VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null | grep '"tag_name"' | head -1 | sed -E 's/.*"tag_name":\s*"([^"]+)".*/\1/' || echo "")
fi

if [ -z "$VERSION" ]; then
    error "could not detect latest version. Check https://github.com/$REPO/releases"
fi

BINARY_NAME="pulley-${VERSION#v}-$OS_NAME-$ARCH_NAME"
DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/$BINARY_NAME"

# ── Download ────────────────────────────────────────────────────────────────

info "Downloading pulley $VERSION for $OS_NAME/$ARCH_NAME..."

TMPDIR=$(mktemp -d)
cleanup() { rm -rf "$TMPDIR"; }
trap cleanup EXIT

curl -fsSL "$DOWNLOAD_URL" -o "$TMPDIR/pulley" || {
    error "download failed. Check https://github.com/$REPO/releases for available binaries."
}

# ── Verify ──────────────────────────────────────────────────────────────────

CHECKSUMS_URL="https://github.com/$REPO/releases/download/$VERSION/checksums-sha256.txt"
if curl -fsSL "$CHECKSUMS_URL" -o "$TMPDIR/checksums.txt" 2>/dev/null; then
    EXPECTED=$(grep "$BINARY_NAME" "$TMPDIR/checksums.txt" | awk '{print $1}')
    if [ -n "$EXPECTED" ]; then
        ACTUAL=$(sha256sum "$TMPDIR/pulley" | awk '{print $1}')
        if [ "$EXPECTED" != "$ACTUAL" ]; then
            error "checksum mismatch! Expected $EXPECTED, got $ACTUAL. File may be corrupted."
        fi
        info "Checksum verified ✓"
    fi
fi

# ── Install binary ──────────────────────────────────────────────────────────

chmod +x "$TMPDIR/pulley"
install -m 755 "$TMPDIR/pulley" "$BINARY"
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

    if ! systemctl is-enabled pulley &>/dev/null; then
        systemctl enable pulley
        info "Service enabled (not started yet)"
    else
        if systemctl is-active pulley &>/dev/null; then
            systemctl restart pulley
            info "Service restarted"
        fi
    fi
fi

# ── Done ────────────────────────────────────────────────────────────────────

INSTALLED_VERSION=$("$BINARY" version 2>&1 | head -1 || true)

echo ""
info "Done! $INSTALLED_VERSION"
echo ""
echo "Quick start:"
echo "  pulley add /path/to/repo"
echo "  pulley add /path/to/repo --interval 15m --branches all"
echo "  pulley config set --interval 15m --range \"08:00-22:00\""
echo "  sudo systemctl start pulley"
echo ""
echo "Update anytime:"
echo "  curl -fsSL https://github.com/Joel-Claw/pulley/releases/latest/download/install.sh | sudo bash"
echo ""
echo "Uninstall:"
echo "  sudo systemctl stop pulley; sudo systemctl disable pulley"
echo "  sudo rm /usr/local/bin/pulley /etc/systemd/system/pulley.service"