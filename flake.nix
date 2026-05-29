{
  description = "BLT Volume Manager - Docker/Podman volume plugin for S3 backup";

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
        # TODO: add formatter output for nix fmt, e.g.:
        #   formatter = pkgs.nixfmt;
        # TODO: commit generated flake.lock to pin inputs

        packages = rec {
          blt-volume-manager = pkgs.buildGoModule {
            pname = "blt-volume-manager";
            version = "0.1.0";
            src = ./.;
            # TODO: replace with actual hash — run `nix build .#blt-volume-manager 2>&1 | grep 'got:'`
            vendorHash = pkgs.lib.fakeSha256;
            ldflags = [ "-s" "-w" ];
            CGO_ENABLED = 0;
            meta = with pkgs.lib; {
              description = "Docker/Podman volume plugin for S3 backup using restic";
              # TODO: set to real repository URL
              homepage = "https://github.com/example/blt-volume-manager";
              # TODO: verify actual project license
              license = licenses.mit;
              platforms = platforms.linux;
            };
          };
          default = blt-volume-manager;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [ go gopls gotools ];
        };
      }
    ) // {
      nixosModules.blt-volume-manager = { config, lib, pkgs, ... }:
        let
          cfg = config.services.blt-volume-manager;
          pkg = self.packages.${pkgs.system}.blt-volume-manager;
        in
        with lib;
        {
          options.services.blt-volume-manager = {
            enable = mkEnableOption "S3 volume plugin (blt-volume-manager)";

            dataDir = mkOption {
              type = types.str;
              default = "/var/lib/docker-volumes";
              description = "Root directory for volumes";
            };

            socketPath = mkOption {
              type = types.str;
              default = "/run/docker/plugins/blt-volume-manager.sock";
              description = "Unix socket path for the Docker/Podman volume plugin";
            };

            httpAddr = mkOption {
              type = types.str;
              default = "";
              description = "HTTP address for the web UI (empty to disable)";
            };

            environment = mkOption {
              type = types.attrsOf types.str;
              default = { };
              description = "Non-sensitive environment variables (safe for the Nix store).";
              example = {
                S3_ENDPOINT = "http://127.0.0.1:3900";
                S3_REGION = "garage";
                RESTIC_REPOSITORY = "s3:http://127.0.0.1:3900/my-bucket";
              };
            };

            environmentFile = mkOption {
              type = types.nullOr types.path;
              default = null;
              description = ''
                Path to a file with secret environment variables (KEY=VALUE lines).
                Use this for RESTIC_PASSWORD, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, etc.
                The file lives outside the Nix store and is read at runtime.
                Example content:
                  RESTIC_PASSWORD=hunter2
                  AWS_ACCESS_KEY_ID=root
                  AWS_SECRET_ACCESS_KEY=secret_key
              '';
            };
          };

          config = mkIf cfg.enable {
            systemd.services.blt-volume-manager = {
              description = "S3 Volume Plugin (blt-volume-manager)";
              wants = [ "network-online.target" ];
              after = [ "network-online.target" ];
              wantedBy = [ "multi-user.target" ];

              path = [ pkgs.restic ];

              environment = cfg.environment;

              # TODO: consider adding restartTriggers for config change detection
              serviceConfig = {
                Type = "simple";
                ExecStart = "${pkg}/bin/blt-volume-manager"
                  + " --data-dir ${cfg.dataDir}"
                  + " --socket ${cfg.socketPath}"
                  + optionalString (cfg.httpAddr != "") " --http-addr ${cfg.httpAddr}";
                EnvironmentFile = lib.optional (cfg.environmentFile != null) cfg.environmentFile;
                Restart = "always";
                RestartSec = "5";
                RuntimeDirectory = [ "docker/plugins" ];
                RuntimeDirectoryMode = "0755";
                NoNewPrivileges = true;
              };
            };
          };
        };
    };
}
