#!/usr/bin/env bash
# autopull installer for Arch Linux
set -euo pipefail

echo "=== autopull installer (Arch Linux) ==="

# Check root
if [ "$(id -u)" -ne 0 ]; then
    echo "error: run as root (sudo ./install-arch.sh)"
    exit 1
fi

# Install dependencies
if ! command -v go &>/dev/null; then
    echo "Installing Go..."
    pacman -S --noconfirm go git
fi

# Build
echo "Building autopull..."
TMPDIR=$(mktemp -d)
cp -r . "$TMPDIR/autopull"
cd "$TMPDIR/autopull"
go build -ldflags="-s -w" -o autopull .

# Install binary
install -m 755 autopull /usr/local/bin/autopull
echo "Installed: /usr/local/bin/autopull"

# Install systemd service
cat > /etc/systemd/system/autopull.service <<'EOF'
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

if [ -n "${SUDO_USER:-}" ]; then
    sed -i "s/\[Service\]/[Service]\nUser=$SUDO_USER\nGroup=$SUDO_USER/" /etc/systemd/system/autopull.service
fi

systemctl daemon-reload
systemctl enable autopull
echo "Installed: /etc/systemd/system/autopull.service"
echo ""
echo "Done! Next steps:"
echo "  autopull add /path/to/repo --interval 15m"
echo "  sudo systemctl start autopull"

rm -rf "$TMPDIR"