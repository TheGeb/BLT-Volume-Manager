{
  description = "BLT Volume Manager - Docker/Podman volume plugin for S3 backup";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    let
      version = builtins.replaceStrings ["\n"] [""] (builtins.readFile ./VERSION);
    in
    flake-utils.lib.eachSystem [ "x86_64-linux" "aarch64-linux" ] (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        ui = pkgs.buildNpmPackage {
          pname = "blt-volume-manager-ui";
          version = version;
          src = ./web/ui;
          npmDepsHash = "sha256-ErqUzOvgdpo1JWp2HL6QjG8SSvx33DV9YhkW6sEVFPo=";
          installPhase = ''
            mkdir -p $out
            cp -r dist/. $out/
          '';
        };

        build-module = { pname, subPackages, meta-description, withUi ? false }:
          pkgs.buildGoModule {
            inherit pname version;
            vendorHash = "sha256-wBQUcIN0m6RUH/4XMAfH6rfFJCHLb7ffpJGbYz2GhJw=";
            src = ./.;

            inherit subPackages;

            preBuild = if withUi then ''
              mkdir -p internal/web/static
              cp -r ${ui}/. internal/web/static/
            '' else ''
              mkdir -p internal/web/static
              touch internal/web/static/.dummy
            '';

            ldflags = [
              "-s"
              "-w"
              "-X github.com/TheGeb/BLT-Volume-Manager/internal/app.Version=v${version}"
              "-X github.com/TheGeb/BLT-Volume-Manager/internal/app.Commit=${self.shortRev or "unknown"}"
              "-X github.com/TheGeb/BLT-Volume-Manager/internal/app.Date=${self.lastModifiedDate or "unknown"}"
            ];
            env.CGO_ENABLED = "0";
            strictDeps = true;

            nativeBuildInputs = [ ];

            meta = with pkgs.lib; {
              inherit meta-description;
              homepage = "https://github.com/TheGeb/docker-s3-volume-plugin";
              platforms = platforms.linux;
            };
          };
      in
      {
        formatter = pkgs.nixfmt;

        packages = rec {
          inherit ui;
          blt-volume-manager = build-module {
            pname = "blt-volume-manager";
            subPackages = [ "cmd/driver" ];
            meta-description = "Docker/Podman volume plugin for S3 backup using restic";
          };
          blt-volume-manager-web = build-module {
            pname = "blt-volume-manager-web";
            subPackages = [ "cmd/web" ];
            meta-description = "BLT Volume Manager web UI";
            withUi = true;
          };
          default = blt-volume-manager;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [ go gopls gotools nodejs gnumake ];
          shellHook = ''
            echo "Run 'make dev ARGS=\"--http-addr :8081\"' to build and run the driver"
          '';
        };

        checks = {
          inherit (self.packages.${system}) blt-volume-manager blt-volume-manager-web;
          default = self.packages.${system}.default;

          go-test = (self.packages.${system}.blt-volume-manager.overrideAttrs (old: {
            doCheck = true;
            preCheck = ''
              mkdir -p internal/web/static
              touch internal/web/static/.dummy
            '';
            installPhase = "touch $out";
          })).overrideAttrs (old: { buildPhase = "true"; checkPhase = ''
            runHook preCheck
            go test ./... -short -count=1
            runHook postCheck
          ''; });

          ui-test = ui.overrideAttrs (old: {
            doCheck = true;
            installPhase = "touch $out";
            checkPhase = ''
              runHook preCheck
              npm test
              runHook postCheck
            '';
          });
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

            user = mkOption {
              type = types.nullOr types.str;
              default = null;
              description = ''
                Run the service as this unprivileged user instead of root.
                Recommended for plain volumes (no filesystem snapshots):
                systemd creates /run/docker/plugins owned by this user and the
                data directory must be writable by it. Set to null to run as
                root (required for filesystem snapshots).
              '';
            };

            filesystemSnapshots = mkOption {
              type = types.bool;
              default = false;
              description = ''
                Enable btrfs/ZFS filesystem snapshots. Requires running as
                root (CAP_SYS_ADMIN), so this forces user = null, and adds
                snapshotTools to the service PATH.
              '';
            };

            snapshotTools = mkOption {
              type = types.listOf types.package;
              default = [ pkgs.btrfs-progs ];
              description = ''
                CLI tools made available to the service for btrfs/ZFS
                snapshots. Add pkgs.zfs on ZFS hosts.
              '';
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
                It is read by the service manager (root), so keep it root-owned
                with restricted permissions, e.g.
                install -m 600 -o root -g root /path/to/file.
                Example content:
                  RESTIC_PASSWORD=hunter2
                  AWS_ACCESS_KEY_ID=root
                  AWS_SECRET_ACCESS_KEY=secret_key
              '';
            };
          };

          config = mkIf cfg.enable {
            assertions = [{
              assertion = !cfg.filesystemSnapshots || cfg.user == null;
              message = "services.blt-volume-manager.filesystemSnapshots requires running as root (set user to null)";
            }];

            systemd.services.blt-volume-manager = {
              description = "S3 Volume Plugin (blt-volume-manager)";
              wants = [ "network-online.target" ];
              after = [ "network-online.target" ];
              wantedBy = [ "multi-user.target" ];

              path = [ pkgs.restic ] ++ optionals cfg.filesystemSnapshots cfg.snapshotTools;

              environment = cfg.environment;

              serviceConfig =
                {
                  Type = "simple";
                  ExecStart =
                    [ "${pkg}/bin/blt-volume-manager"
                      "--data-dir" cfg.dataDir
                      "--socket" cfg.socketPath
                    ] ++ optional (cfg.httpAddr != "") [
                      "--http-addr" cfg.httpAddr
                    ];
                  EnvironmentFile = lib.optional (cfg.environmentFile != null) cfg.environmentFile;
                  Restart = "always";
                  RestartSec = "5";
                  RuntimeDirectory = [ "docker/plugins" ];
                  RuntimeDirectoryMode = "0755";
                  NoNewPrivileges = true;
                  ProtectSystem = "full";
                  PrivateTmp = true;
                  ProtectKernelTunables = true;
                  ProtectKernelModules = true;
                  ProtectControlGroups = true;
                  SystemCallArchitectures = "native";
                  LockPersonality = true;
                  LimitCORE = 0;
                }
                // optionalAttrs (cfg.user != null) {
                  User = cfg.user;
                  Group = cfg.user;
                  CapabilityBoundingSet = [ ];
                  PrivateDevices = true;
                };
            };
          };
        };

      # Snapshots variant — same module with filesystemSnapshots enabled.
      nixosModules.blt-volume-manager-snapshots = { lib, ... }: {
        imports = [ self.nixosModules.blt-volume-manager ];
        services.blt-volume-manager.filesystemSnapshots = lib.mkDefault true;
      };
    };
}
