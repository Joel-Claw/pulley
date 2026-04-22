#!/usr/bin/env bash
# autopull installer for NixOS / Nix
set -euo pipefail

echo "=== autopull installer (Nix) ==="

if ! command -v nix &>/dev/null; then
    echo "error: nix not found. Install Nix first: https://nixos.org/download"
    exit 1
fi

# Option 1: Install as a profile package (works on any Nix installation)
echo ""
echo "Choose installation method:"
echo "  1) nix profile install (any Nix system)"
echo "  2) Add to NixOS configuration (NixOS only)"
echo "  3) nix run (one-time, no install)"
echo ""
read -rp "Choice [1/2/3]: " choice

case "$choice" in
    1)
        echo "Installing autopull to user profile..."
        nix profile install "$(pwd)#default"
        echo "Done! Run: autopull add /path/to/repo"
        ;;
    2)
        echo ""
        echo "Add the following to your configuration.nix:"
        echo ""
        echo "  imports = [ $(pwd)/flake.nix#nixosModules.autopull ];"
        echo "  services.autopull.enable = true;"
        echo "  services.autopull.user = \"youruser\";"
        echo ""
        echo "Then: sudo nixos-rebuild switch"
        ;;
    3)
        echo "Running autopull without installing..."
        nix run "$(pwd)#default" -- "$@"
        ;;
    *)
        echo "Invalid choice"
        exit 1
        ;;
esac