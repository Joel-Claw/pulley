# AutoPull Nix flake
# Install: nix profile install github:Joel-Claw/autopull
# Run service: systemd.services.autopull (see below)

{
  description = "AutoPull - Automatic Git Pull Daemon";

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
          pname = "autopull";
          version = "0.1.0";
          src = ./.;
          vendorHash = null; # no external dependencies
          meta = with pkgs.lib; {
            description = "A lightweight Linux service that automatically git pulls registered repositories on a configurable schedule";
            homepage = "https://github.com/Joel-Claw/autopull";
            license = licenses.mit;
            platforms = platforms.linux;
          };
        };

        # NixOS module for running autopull as a systemd service
        nixosModules.autopull = { config, lib, pkgs, ... }:
          with lib;
          let
            cfg = config.services.autopull;
          in
          {
            options.services.autopull = {
              enable = mkEnableOption "autopull daemon";
              user = mkOption {
                type = types.str;
                default = "root";
                description = "User to run autopull as";
              };
              configPath = mkOption {
                type = types.str;
                default = "/etc/autopull/config.json";
                description = "Path to autopull config file";
              };
            };

            config = mkIf cfg.enable {
              systemd.services.autopull = {
                description = "AutoPull - Automatic Git Pull Daemon";
                after = [ "network-online.target" ];
                wants = [ "network-online.target" ];
                wantedBy = [ "multi-user.target" ];
                serviceConfig = {
                  Type = "simple";
                  ExecStart = "${pkgs.autopull}/bin/autopull daemon";
                  User = cfg.user;
                  Restart = "on-failure";
                  RestartSec = "30";
                };
                environment = {
                  XDG_CONFIG_HOME = dirOf cfg.configPath;
                };
              };

              environment.systemPackages = [ pkgs.autopull ];
            };
          };
      }
    );
}