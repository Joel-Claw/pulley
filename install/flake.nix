# Pulley Nix flake
# Install: nix profile install github:Joel-Claw/pulley
# Run service: systemd.services.pulley (see below)

{
  description = "Pulley - Automatic Git Pull Daemon";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "pulley";
          version = "0.1.0";
          src = ./.;
          vendorHash = null; # no external dependencies
          meta = with pkgs.lib; {
            description = "A lightweight Linux service that automatically git pulls registered repositories on a configurable schedule";
            homepage = "https://github.com/Joel-Claw/pulley";
            license = licenses.mit;
            platforms = platforms.linux;
          };
        };

        # NixOS module for running pulley as a systemd service
        nixosModules.pulley = { config, lib, pkgs, ... }:
          with lib;
          let
            cfg = config.services.pulley;
          in
          {
            options.services.pulley = {
              enable = mkEnableOption "pulley daemon";
              user = mkOption {
                type = types.str;
                default = "root";
                description = "User to run pulley as";
              };
              configPath = mkOption {
                type = types.str;
                default = "/etc/pulley/config.json";
                description = "Path to pulley config file";
              };
            };

            config = mkIf cfg.enable {
              systemd.services.pulley = {
                description = "Pulley - Automatic Git Pull Daemon";
                after = [ "network-online.target" ];
                wants = [ "network-online.target" ];
                wantedBy = [ "multi-user.target" ];
                serviceConfig = {
                  Type = "simple";
                  ExecStart = "${pkgs.pulley}/bin/pulley daemon";
                  User = cfg.user;
                  Restart = "on-failure";
                  RestartSec = "30";
                };
                environment = {
                  XDG_CONFIG_HOME = dirOf cfg.configPath;
                };
              };

              environment.systemPackages = [ pkgs.pulley ];
            };
          };
      }
    );
}