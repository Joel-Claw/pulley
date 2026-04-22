#!/usr/bin/env bash
# pulley installer for Debian/Ubuntu
set -euo pipefail

VERSION="${1:-latest}"
BINARY="/usr/local/bin/pulley"
SERVICE_FILE="/etc/systemd/system/pulley.service"
CONFIG_DIR="/home"

echo "=== pulley installer (Debian/Ubuntu) ==="

# Check root
if [ "$(id -u)" -ne 0 ]; then
    echo "error: run as root (sudo ./install-debian.sh)"
    exit 1
fi

# Check git
if ! command -v git &>/dev/null; then
    echo "error: git is required but not installed. Install it first:"
    echo "  sudo apt install git"
    exit 1
fi

# Install dependencies
if ! command -v go &>/dev/null; then
    echo "Installing Go..."
    apt-get update -qq
    apt-get install -y -qq golang-go
fi

# Build
echo "Building pulley..."
TMPDIR=$(mktemp -d)
cp -r . "$TMPDIR/pulley"
cd "$TMPDIR/pulley"
go build -ldflags="-s -w" -o pulley .
strip pulley 2>/dev/null || true

# Install binary
install -m 755 pulley "$BINARY"
echo "Installed: $BINARY"

# Install systemd service
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

# Set user in service if SUDO_USER is set
if [ -n "${SUDO_USER:-}" ]; then
    sed -i "s/\[Service\]/[Service]\nUser=$SUDO_USER\nGroup=$SUDO_USER/" "$SERVICE_FILE"
fi

systemctl daemon-reload
echo "Installed: $SERVICE_FILE"

# Enable but don't start yet (user needs to add repos first)
systemctl enable pulley
echo ""
echo "Done! Next steps:"
echo "  pulley add /path/to/repo --interval 15m"
echo "  sudo systemctl start pulley"

# Cleanup
rm -rf "$TMPDIR"