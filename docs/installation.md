# Installation

BLT Volume Manager can be installed four ways — as a **host binary**, as a
**container image**, via **`docker plugin install`**, or via **Nix** — and each
works in both **rooted** (rootful) and **rootless** Docker. The table below
summarizes what changes between modes.

| | Rooted Docker | Rootless Docker |
|---|---|---|
| Daemon runs as | root | your unprivileged user |
| Plugin socket dir | `/run/docker/plugins` | `$XDG_RUNTIME_DIR/docker/plugins` (usually `/run/user/$UID/docker/plugins`) |
| Plugin socket | `/run/docker/plugins/blt-volume-manager.sock` | `$XDG_RUNTIME_DIR/docker/plugins/blt-volume-manager.sock` |
| Data dir | `/var/lib/docker-volumes` | `$XDG_DATA_HOME/blt-volume-manager` (default `~/.local/share/blt-volume-manager`) |
| Plugin runs as | root (or root-capable) | the same user as the daemon |
| `BLT_LISTEN` needed? | No | Only when running the **binary** directly |

`docker plugin install` (the image-based, privileged plugin path) is **root-only
and not supported under rootless Docker** — see [section 3](#3-docker-plugin-install-root-only).
For rootless, use the binary (section 1) or container (section 2) methods.

## Prerequisites (all methods)

- **restic ≥ v0.17.0** (see README). The container image pins restic 0.19.1;
  for binary/Nix installs make sure restic is on `PATH`.
- Configuration via environment variables (`RESTIC_REPOSITORY`,
  `RESTIC_PASSWORD`, AWS/S3 credentials, etc.) — see the README's
  [Configuration section](../README.md#configuration).
- Rootless Docker prerequisites: `newuidmap`/`newgidmap`, and `subuid`/`subgid`
  ranges for your user. See the
  [Docker rootless docs](https://docs.docker.com/engine/security/rootless/).

## 1. Host binary

### Rooted

Build (or download) the driver binary, then run it as root:

```bash
make build-release        # produces ./blt-volume-manager-plugin
sudo mkdir -p /run/docker/plugins /var/lib/docker-volumes
sudo ./blt-volume-manager-plugin \
  --data-dir /var/lib/docker-volumes \
  --socket /run/docker/plugins/blt-volume-manager.sock
```

Run it as a system service:

```ini
# /etc/systemd/system/blt-volume-manager.service
[Unit]
Description=BLT Volume Manager (S3 volume plugin)
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=/etc/blt-volume-manager.env
ExecStart=/usr/local/bin/blt-volume-manager-plugin \
  --data-dir /var/lib/docker-volumes \
  --socket /run/docker/plugins/blt-volume-manager.sock
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now blt-volume-manager
```

### Rooted — non-root service account

For **plain volumes the plugin needs no root at all**: the Docker daemon
performs the actual bind mount, while the plugin only creates directories,
runs restic, and writes its socket. So even on a rooted daemon you can run the
plugin as a dedicated unprivileged user. The only root-owned resource it needs
is the socket directory `/run/docker/plugins` — have systemd create it owned
by the service user with `RuntimeDirectory`:

```ini
# /etc/systemd/system/blt-volume-manager.service
[Unit]
Description=BLT Volume Manager (S3 volume plugin)
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
User=blt
Group=blt
# systemd creates /run/docker/plugins owned by the service user
RuntimeDirectory=docker/plugins
RuntimeDirectoryMode=0755
EnvironmentFile=/etc/blt-volume-manager.env
ExecStart=/usr/local/bin/blt-volume-manager-plugin \
  --data-dir /var/lib/docker-volumes \
  --socket /run/docker/plugins/blt-volume-manager.sock
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo useradd -r -d /var/lib/blt blt
sudo mkdir -p /var/lib/docker-volumes
sudo chown blt:blt /var/lib/docker-volumes
sudo systemctl daemon-reload
sudo systemctl enable --now blt-volume-manager
```

The root daemon can still connect to the `blt`-owned socket, and containers
bind-mount the `blt`-owned data dir. The equivalent with a tmpfiles rule is:
`d /run/docker/plugins 0755 blt blt -`.

> Caveat: btrfs/ZFS snapshots need real root (see [section 5](#5-filesystem-snapshots-btrfszfs)).
> The service-account setup covers plain volumes only.

### Rootless

The plugin **must run as the same user that runs the rootless daemon**. Two
things differ from rooted: the socket goes under `$XDG_RUNTIME_DIR/docker/plugins`,
and you must set `BLT_LISTEN=1`.

`BLT_LISTEN` is required because the vendored Docker plugin helper hardcodes
`os.MkdirAll("/run/docker/plugins", 0755)` before listening; a non-root user
cannot create that directory. The `BLT_LISTEN` escape hatch uses `net.Listen`
directly so the socket can live anywhere the user owns.

```bash
export XDG_RUNTIME_DIR="/run/user/$(id -u)"

mkdir -p "$XDG_RUNTIME_DIR/docker/plugins" \
         "$HOME/.local/share/blt-volume-manager"

BLT_LISTEN=1 ./blt-volume-manager-plugin \
  --data-dir "$HOME/.local/share/blt-volume-manager" \
  --socket "$XDG_RUNTIME_DIR/docker/plugins/blt-volume-manager.sock"
```

Run it as a per-user service (keep it alive after logout):

```ini
# ~/.config/systemd/user/blt-volume-manager.service
[Unit]
Description=BLT Volume Manager (S3 volume plugin)
After=docker.service

[Service]
Type=simple
Environment=BLT_LISTEN=1
EnvironmentFile=%h/.config/blt-volume-manager/env
ExecStartPre=mkdir -p %t/docker/plugins
ExecStart=%h/bin/blt-volume-manager-plugin \
  --data-dir %h/.local/share/blt-volume-manager \
  --socket %t/docker/plugins/blt-volume-manager.sock
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
```

```bash
loginctl enable-linger "$USER"
systemctl --user daemon-reload
systemctl --user enable --now blt-volume-manager
```

## 2. Container image

This runs the `plugin` image target as a normal container that shares the
plugin socket and data directories with the host daemon. No `BLT_LISTEN` is
needed here: inside the container the plugin runs as root, so the helper's
`MkdirAll("/run/docker/plugins")` succeeds — the bind mount is what makes the
socket visible to the host.

Build locally, or use a published release image
(`ghcr.io/thegeb/blt-volume-manager-plugin:v<VERSION>`):

```bash
docker build --target plugin -t blt-volume-manager-plugin:local .
```

### Rooted

```bash
docker run -d --name blt-volume-manager --restart always \
  -v /run/docker/plugins:/run/docker/plugins \
  -v /var/lib/docker-volumes:/var/lib/docker-volumes \
  --env-file /etc/blt-volume-manager.env \
  blt-volume-manager-plugin:local
```

Only add capabilities/devices if you plan to create **btrfs/ZFS** volumes (see
[section 5](#5-filesystem-snapshots-btrfszfs)). Plain volumes require no
privileges. Grant the narrowest set instead of `--privileged`: `CAP_SYS_ADMIN`
covers btrfs, and `/dev/zfs` is additionally needed for ZFS:

```bash
# btrfs only
docker run -d --name blt-volume-manager --restart always \
  --cap-add SYS_ADMIN \
  -v /run/docker/plugins:/run/docker/plugins \
  -v /var/lib/docker-volumes:/var/lib/docker-volumes \
  --env-file /etc/blt-volume-manager.env \
  blt-volume-manager-plugin:local

# ZFS — also pass the device node (omit this on hosts without ZFS,
# since --device fails if the path doesn't exist)
docker run -d --name blt-volume-manager --restart always \
  --cap-add SYS_ADMIN --device /dev/zfs:rwm \
  -v /run/docker/plugins:/run/docker/plugins \
  -v /var/lib/docker-volumes:/var/lib/docker-volumes \
  --env-file /etc/blt-volume-manager.env \
  blt-volume-manager-plugin:local
```

This keeps the default seccomp profile enforced (it allows `mount` only
because the process holds `CAP_SYS_ADMIN`) and grants no other capabilities or
devices. Only fall back to `--privileged` if snapshot operations still fail —
it's the broad envelope, not the default.

### Rootless

```bash
export XDG_RUNTIME_DIR="/run/user/$(id -u)"
mkdir -p "$XDG_RUNTIME_DIR/docker/plugins" \
         "$HOME/.local/share/blt-volume-manager"

docker run -d --name blt-volume-manager --restart always \
  -v "$XDG_RUNTIME_DIR/docker/plugins":/run/docker/plugins \
  -v "$HOME/.local/share/blt-volume-manager":/var/lib/docker-volumes \
  --env-file "$HOME/.config/blt-volume-manager.env" \
  blt-volume-manager-plugin:local
```

The socket file is created by container root, which maps to your UID in the
user namespace — so the rootless daemon (same user) can access it.

> In rootless, `--privileged` does **not** enable btrfs/ZFS: it only grants
> capabilities inside the user namespace, not host mount privileges. btrfs/ZFS
> remain unsupported under rootless (see [section 5](#5-filesystem-snapshots-btrfszfs)).

## 3. `docker plugin install` (root-only)

This is Docker's native, image-based plugin mechanism. It only works with
**rooted** Docker — rootless does not support `docker plugin install`. The
plugin runs as root with only the privileges its manifest declares.

The repository does not currently ship a ready-made plugin image, but you can
build one from the existing `plugin` image target. A Docker plugin image is a
`config.json` plus a `rootfs/` directory, assembled with `docker plugin create`.

### Build the plugin image locally

```bash
# 1. Build the plugin container and extract its rootfs
docker build --target plugin -t blt-volume-manager-plugin:local .
cid=$(docker create blt-volume-manager-plugin:local)
mkdir -p /tmp/blt-plugin/rootfs
docker export "$cid" | sudo tar -x -C /tmp/blt-plugin/rootfs
docker rm "$cid"

# 2. Write the plugin manifest
cat > /tmp/blt-plugin/config.json <<'EOF'
{
  "description": "BLT Volume Manager - S3 backup volume plugin",
  "entrypoint": ["/usr/local/bin/blt-volume-manager"],
  "network": { "type": "host" },
  "linux": {
    "capabilities": ["CAP_SYS_ADMIN"],
    "mounts": [
      {
        "name": "data-dir",
        "type": "bind",
        "destination": "/var/lib/docker-volumes",
        "options": ["rbind", "rw"]
      }
    ]
  }
}
EOF

# 3. Create the plugin from the staging dir
docker plugin create blt-volume-manager /tmp/blt-plugin
```

Notes:
- `/run/docker/plugins` is provisioned automatically by the plugin framework;
  you don't declare it.
- For bind mounts Docker uses the destination path as the host source, so
  `/var/lib/docker-volumes` inside the plugin is the same directory the daemon
  bind-mounts into containers.
- `network.host` is required so restic can reach S3.
- `CAP_SYS_ADMIN` is only needed for btrfs/ZFS snapshots
  ([section 5](#5-filesystem-snapshots-btrfszfs)). For plain volumes you can
  omit the `capabilities` key.
- The `plugin` image target already bundles restic in the rootfs.

### Configure and enable

Env vars are baked in at install time, but **every var must be declared in the
plugin's `config.json` `env` array to be settable** (undeclared ones are
rejected). Declare all the keys you'll use, e.g.:

```json
"env": [
  { "name": "RESTIC_REPOSITORY", "description": "Restic repository URL", "settable": ["value"] },
  { "name": "RESTIC_PASSWORD",   "description": "Restic repo password",  "settable": ["value"] },
  { "name": "AWS_ACCESS_KEY_ID",     "description": "AWS access key",     "settable": ["value"] },
  { "name": "AWS_SECRET_ACCESS_KEY", "description": "AWS secret key",     "settable": ["value"] },
  { "name": "S3_ENDPOINT",       "description": "S3 endpoint",        "settable": ["value"] }
]
```

Then set them on the disabled plugin and enable it:

```bash
docker plugin set blt-volume-manager \
  RESTIC_REPOSITORY=s3:http://127.0.0.1:3900/backups \
  RESTIC_PASSWORD=changeme \
  AWS_ACCESS_KEY_ID=... \
  AWS_SECRET_ACCESS_KEY=...

docker plugin enable blt-volume-manager
docker plugin ls
```

**Config-file fallback (recommended).** The driver loads KEY=VALUE files and
lets real environment variables win. Set `BLT_CONFIG_FILE` (declare it in the
`env` array) and mount a config file into the plugin so settings live on the
host and don't require a reinstall to change:

```json
"env": [ { "name": "BLT_CONFIG_FILE", "description": "Path to a KEY=VALUE config file", "settable": ["value"] } ],
"linux": {
  "mounts": [
    { "name": "config", "type": "bind", "destination": "/etc/blt-volume-manager.env", "options": ["rbind", "ro"] },
    { "name": "data-dir", "type": "bind", "destination": "/var/lib/docker-volumes", "options": ["rbind", "rw"] }
  ]
}
```

```bash
# on the host, before enabling
sudo tee /etc/blt-volume-manager.env >/dev/null <<'EOF'
RESTIC_REPOSITORY=s3:http://127.0.0.1:3900/backups
RESTIC_PASSWORD=changeme
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...
EOF

docker plugin set blt-volume-manager BLT_CONFIG_FILE=/etc/blt-volume-manager.env
docker plugin enable blt-volume-manager
```

Precedence: process env (from `docker plugin set`) > `./.env` > `BLT_CONFIG_FILE`.

The plugin logs a warning (`config_file_permissions_loose`) if the config file
is accessible by other users (any world read/write bit set); it still loads the
file, so this is advisory — `chmod 600` the file to silence it.

When `docker plugin enable` runs, Docker may prompt you to accept the plugin's
declared privileges (the mounts, `env`, and `CAP_SYS_ADMIN`) before starting
it — confirm them.

### Install from a registry

To distribute and install on other hosts, push the plugin to a registry and
use `docker plugin install` there. Put secrets in a mounted config file (above)
so only `BLT_CONFIG_FILE` needs to be passed:

```bash
docker plugin push ghcr.io/thegeb/blt-volume-manager-plugin:latest

# on the target host
docker plugin install --grant-all-permissions \
  --env BLT_CONFIG_FILE=/etc/blt-volume-manager.env \
  ghcr.io/thegeb/blt-volume-manager-plugin:latest
```

`--grant-all-permissions` skips the interactive privilege prompt.

### Use

```bash
docker volume create -d blt-volume-manager --name my-vol
docker volume ls
```

### Caveats

- Requires root; unsupported under rootless Docker — use section 1 or 2 there.
- Only env vars declared in the plugin's `config.json` `env` array are settable.
- Env vars are baked in at install time, but the `BLT_CONFIG_FILE` fallback
  (above) avoids reinstalls — put mutable settings in the mounted config file.
- btrfs/ZFS inside a plugin image are especially fragile (namespace, device,
  and mount propagation constraints). Prefer the host binary (section 1) for
  filesystem-snapshot volumes.

## 4. Nix

### Rooted — NixOS module

The flake exports a NixOS module that installs the driver as a hardened
systemd service. It supports two privilege postures via the `user` option:

- **Plain volumes (default, unprivileged):** set `user` to a service account —
  the plugin needs no privileges, and systemd creates `/run/docker/plugins`
  owned by that user. This is the least-privilege setup.
- **btrfs/ZFS snapshots:** runs as root (CAP_SYS_ADMIN). Use the
  `blt-volume-manager-snapshots` module variant below.

Plain volumes, unprivileged:

```nix
# flake.nix (config module)
{
  inputs.blt-volume-manager.url = "github:TheGeb/docker-s3-volume-plugin";
}

# configuration.nix
{
  imports = [ blt-volume-manager.nixosModules.blt-volume-manager ];

  services.blt-volume-manager = {
    enable = true;
    dataDir = "/var/lib/docker-volumes";
    socketPath = "/run/docker/plugins/blt-volume-manager.sock";

    # Run unprivileged (recommended for plain volumes)
    user = "blt";

    # Non-secret settings (safe for the Nix store)
    environment = {
      RESTIC_REPOSITORY = "s3:http://127.0.0.1:3900/backups";
      S3_REGION = "garage";
      S3_FORCE_PATH_STYLE = "1";
    };

    # Secrets (KEY=VALUE lines, read at runtime, outside the store)
    environmentFile = "/var/lib/blt-volume-manager.env";
  };
}
```

Create the service account and give it the data dir, and create the secret
env file root-owned with restricted perms (run once):

```bash
sudo useradd -r -d /var/lib/blt blt
sudo mkdir -p /var/lib/docker-volumes
sudo chown blt:blt /var/lib/docker-volumes

# secrets are read by the service manager (root), never bake them into
# `environment` (that lands in the world-readable Nix store / unit file)
sudo install -m 600 -o root -g root /dev/stdin /var/lib/blt-volume-manager.env <<'EOF'
RESTIC_PASSWORD=changeme
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...
EOF

sudo nixos-rebuild switch --flake github:TheGeb/docker-s3-volume-plugin
```

btrfs/ZFS snapshots (root):

```nix
# configuration.nix
{
  imports = [ blt-volume-manager.nixosModules.blt-volume-manager-snapshots ];

  services.blt-volume-manager = {
    enable = true;
    dataDir = "/var/lib/docker-volumes";

    # Default is [ pkgs.btrfs-progs ]; add pkgs.zfs on ZFS hosts
    snapshotTools = [ pkgs.btrfs-progs pkgs.zfs ];
  };
}
```

The `-snapshots` variant enables `filesystemSnapshots` (which forces `user =
null`/root and puts the snapshot CLIs on `PATH`). Both variants share the same
package and options; pick the snapshots one only if you actually create
btrfs/ZFS volumes (see [section 5](#5-filesystem-snapshots-btrfszfs)).

The module's `RuntimeDirectory = [ "docker/plugins" ]` guarantees
`/run/docker/plugins` exists before the service starts, so no `BLT_LISTEN` is
required.

> **Note:** the module only configures the volume **driver**. The web UI is a
> separate binary (`blt-volume-manager-web`); do not set the module's
> `httpAddr` option, as it is passed to the driver which does not support
> `--http-addr`.

### Rootless — package + user service

Install the driver package and run it as a per-user service (the flake does
not provide a rootless module):

```bash
nix profile install github:TheGeb/docker-s3-volume-plugin
# provides ~/.nix-profile/bin/blt-volume-manager
```

```ini
# ~/.config/systemd/user/blt-volume-manager.service
[Unit]
Description=BLT Volume Manager (S3 volume plugin)
After=docker.service

[Service]
Type=simple
Environment=BLT_LISTEN=1
EnvironmentFile=%h/.config/blt-volume-manager/env
ExecStartPre=mkdir -p %t/docker/plugins
ExecStart=%h/.nix-profile/bin/blt-volume-manager \
  --data-dir %h/.local/share/blt-volume-manager \
  --socket %t/docker/plugins/blt-volume-manager.sock
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
```

```bash
loginctl enable-linger "$USER"
systemctl --user daemon-reload
systemctl --user enable --now blt-volume-manager
```

## 5. Filesystem snapshots (btrfs/ZFS)

BLT Volume Manager can use **btrfs** or **ZFS** subvolume/dataset snapshots for
crash-consistent cold backups and pre-restore snapshots instead of backing up
the live directory.

### When the btrfs/ZFS code runs

- Only for volumes created with the `btrfs=true` or `zfs=true` driver option
  (see below). **Plain volumes are ordinary directories and never touch
  btrfs/ZFS code** — even if the data dir happens to sit on a btrfs/ZFS
  filesystem.
- The btrfs provider shells out to the `btrfs` CLI (`subvolume create /
  snapshot / delete`); the ZFS provider shells out to `zfs` (`create /
  snapshot / destroy`) and to real `mount`/`umount`, and needs `/dev/zfs`.
- Failures degrade gracefully: a volume that can't be initialized becomes a
  plain directory, and cold backups fall back to a direct restic backup. These
  are logged, never fatal.

### Compatibility

| Setup | btrfs/ZFS volumes | Plain volumes |
|---|---|---|
| Rooted host binary / Nix (root, section 1/4) | Work | Work |
| Rooted container `docker run` (section 2) | Need `--cap-add SYS_ADMIN` (+ `--device /dev/zfs` for ZFS) | Work |
| `docker plugin install` (section 3) | Need `CAP_SYS_ADMIN` in manifest | Work |
| Rootless — any method | **Not supported** | Work |

Rootless cannot use them: btrfs subvolume operations need CAP_SYS_ADMIN, ZFS
snapshots need a real mount (`mount -t zfs`) and `/dev/zfs`, and a rootless
process has neither. `--privileged` does not help in rootless — it only grants
capabilities inside the user namespace, not host mount privileges. A volume
created with `btrfs=true`/`zfs=true` under rootless silently degrades to a
plain directory.

### Using btrfs/ZFS volumes (rooted)

Prerequisites:
- Data dir on the matching filesystem (on a btrfs subvolume, or under a ZFS
  dataset).
- `btrfs` or `zfs` CLI installed, and for ZFS a usable `/dev/zfs`.
- Plugin running with root (host binary/systemd or NixOS module), or a
  container granted `--cap-add SYS_ADMIN` (plus `/dev/zfs` for ZFS, section 2)
  / a `CAP_SYS_ADMIN` plugin manifest.

```bash
# btrfs — data dir must live on a btrfs filesystem
docker volume create -d blt-volume-manager --name app-data -o btrfs=true

# zfs — optionally pin a specific parent dataset
docker volume create -d blt-volume-manager --name app-data \
  -o zfs=true -o zfs-pool=tank/volumes
```

The driver reads the `btrfs`/`zfs`/`zfs-pool` options in `initFsType`
(`internal/driver/api.go`). Once a volume has an fs type, its cold backups and
pre-restore snapshots use filesystem snapshots automatically.

### Least privilege / reducing CAP_SYS_ADMIN risk

btrfs subvolume and snapshot ioctls are gated on `CAP_SYS_ADMIN` in the kernel,
and so is the `mount(2)` call ZFS snapshot access relies on — there is no
finer-grained capability for either. Since `CAP_SYS_ADMIN` is a broad umbrella
(mounts, namespaces, etc.), prefer these alternatives when you can:

- **Skip filesystem snapshots (zero privileges).** Plain volumes need no
  capabilities at all; the plugin already falls back to a direct restic backup
  when a snapshot can't be created (`internal/driver/backup.go`). You only lose
  btrfs/ZFS crash-consistency.
- **ZFS delegation (host binary only).** `zfs allow` grants a specific user
  specific dataset permissions without root:
  ```bash
  sudo zfs allow -d -u blt create,snapshot,destroy,mount tank/volumes
  ```
  Run the plugin as `blt` (see section 1, non-root service account). The
  delegated user also needs access to `/dev/zfs`. This does **not** work inside
  a container or `docker plugin install` — delegation is checked against host
  process credentials, not container capabilities.
- **Privilege-separated sidecar.** Give `CAP_SYS_ADMIN` only to a tiny,
  single-purpose process that performs snapshot create/delete over a unix
  socket; keep the main plugin unprivileged. This confines the dangerous cap to
  minimal code instead of the whole plugin.
- **If you must grant it in a container/plugin**, keep the blast radius as
  small as the manifest allows: declare only `["CAP_SYS_ADMIN"]`,
  `allowAllDevices: false` with exactly `/dev/zfs` listed, no host PID/IPC
  access, and a read-only rootfs — do not reach for the whole "privileged"
  envelope.

## Verify the plugin works

```bash
docker volume create -d blt-volume-manager --name smoke-test
docker volume ls                              # shows smoke-test
docker run --rm -v smoke-test:/data alpine sh -c 'echo hi > /data/hello && cat /data/hello'
docker volume rm smoke-test
```

Check plugin logs:

```bash
# binary / container
journalctl -u blt-volume-manager              # or: docker logs blt-volume-manager
```

## Troubleshooting

| Symptom | Likely cause / fix |
|---|---|
| `permission denied` creating the socket or `/run/docker/plugins` | Plugin started as wrong user. In rootless it must be the daemon's user, socket under `$XDG_RUNTIME_DIR/docker/plugins`, with `BLT_LISTEN=1`. |
| Plugin starts but Docker can't find it | Socket is in the wrong dir. Rooted: `/run/docker/plugins`. Rootless: `$XDG_RUNTIME_DIR/docker/plugins`. |
| Works, but container can't write to a mounted volume | Data dir permissions. In rootless the plugin and daemon both run as your user, so keep the data dir under your home. |
| `docker plugin install` fails | `docker plugin install` is root-only and unsupported in rootless — use the container (section 2) or binary (section 1) method there. |
| btrfs/ZFS volume fell back to a plain dir | Missing privileges or CLI. Rooted host binary: install `btrfs`/`zfs` and run as root. Container: add `--cap-add SYS_ADMIN` (+ `--device /dev/zfs` for ZFS), or `--privileged` as a fallback (section 2). Plugin: declare `CAP_SYS_ADMIN` (section 3). Rootless: unsupported — use plain volumes (section 5). |
| `btrfs: command not found` / `zfs: command not found` | Install the CLI on the host, or add it to the container/plugin rootfs (section 5). |
| Log: `config_file_permissions_loose` | The `BLT_CONFIG_FILE` file is world-accessible; `chmod 600` it (the driver still loads it). |
| Plugin stops after logout | Enable lingering: `loginctl enable-linger "$USER"`. |
