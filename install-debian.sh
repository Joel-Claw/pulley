#!/usr/bin/env bash
# autopull installer for Debian/Ubuntu
set -euo pipefail

VERSION="${1:-latest}"
BINARY="/usr/local/bin/autopull"
SERVICE_FILE="/etc/systemd/system/autopull.service"
CONFIG_DIR="/home"

echo "=== autopull installer (Debian/Ubuntu) ==="

# Check root
if [ "$(id -u)" -ne 0 ]; then
    echo "error: run as root (sudo ./install-debian.sh)"
    exit 1
fi

# Install dependencies
if ! command -v go &>/dev/null; then
    echo "Installing Go..."
    apt-get update -qq
    apt-get install -y -qq golang-go git
fi

# Build
echo "Building autopull..."
TMPDIR=$(mktemp -d)
cp -r . "$TMPDIR/autopull"
cd "$TMPDIR/autopull"
go build -ldflags="-s -w" -o autopull .
strip autopull 2>/dev/null || true

# Install binary
install -m 755 autopull "$BINARY"
echo "Installed: $BINARY"

# Install systemd service
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

# Set user in service if SUDO_USER is set
if [ -n "${SUDO_USER:-}" ]; then
    sed -i "s/\[Service\]/[Service]\nUser=$SUDO_USER\nGroup=$SUDO_USER/" "$SERVICE_FILE"
fi

systemctl daemon-reload
echo "Installed: $SERVICE_FILE"

# Enable but don't start yet (user needs to add repos first)
systemctl enable autopull
echo ""
echo "Done! Next steps:"
echo "  autopull add /path/to/repo --interval 15m"
echo "  sudo systemctl start autopull"

# Cleanup
rm -rf "$TMPDIR"