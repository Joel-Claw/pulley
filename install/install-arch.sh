#!/usr/bin/env bash
# pulley installer for Arch Linux
set -euo pipefail

echo "=== pulley installer (Arch Linux) ==="

# Check root
if [ "$(id -u)" -ne 0 ]; then
    echo "error: run as root (sudo ./install-arch.sh)"
    exit 1
fi

# Check git
if ! command -v git &>/dev/null; then
    echo "error: git is required but not installed. Install it first:"
    echo "  sudo pacman -S git"
    exit 1
fi

# Install dependencies
if ! command -v go &>/dev/null; then
    echo "Installing Go..."
    pacman -S --noconfirm go
fi

# Build
echo "Building pulley..."
TMPDIR=$(mktemp -d)
cp -r . "$TMPDIR/pulley"
cd "$TMPDIR/pulley"
go build -ldflags="-s -w" -o pulley .

# Install binary
install -m 755 pulley /usr/local/bin/pulley
echo "Installed: /usr/local/bin/pulley"

# Install systemd service
cat > /etc/systemd/system/pulley.service <<'EOF'
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

if [ -n "${SUDO_USER:-}" ]; then
    sed -i "s/\[Service\]/[Service]\nUser=$SUDO_USER\nGroup=$SUDO_USER/" /etc/systemd/system/pulley.service
fi

systemctl daemon-reload
systemctl enable pulley
echo "Installed: /etc/systemd/system/pulley.service"
echo ""
echo "Done! Next steps:"
echo "  pulley add /path/to/repo --interval 15m"
echo "  sudo systemctl start pulley"

rm -rf "$TMPDIR"