# BLT Volume Manager

**Backup + Lock + Transfer** — A Docker volume plugin that uses restic to backup volumes to S3 and uses ownership to coordinate access between multiple hosts.

Features

- Implements Docker volume plugin methods: Create, Remove, Mount, Unmount, Path, Get, List, Capabilities
- Uses a pluggable `owner` interface. The default is a simple file owner; replace with an S3-based owner for cross-host ownership.
- Uses `restic` (external binary) to perform hot and cold backups on configurable schedules.

Restore selection (hot vs cold)

- Backups are tagged in restic as `hot` or `cold` (the manager uses `--tag`).
- When calling `Create` you can pass an option `restore=hot|cold|latest` to control which snapshot to restore on creation.
- The plugin currently restores `latest` by default if `restore` is not provided.

S3 owner (optional)

- To enable an S3-based cross-host owner, provide `OWNER_LOCK_S3_BUCKET` environment variable.
- Owner keys are stored under `blt-volume-manager/owners/<volume>/` within the configured bucket.

Environment variables used by the S3 owner:

- `S3_ENDPOINT` — optional custom S3 endpoint (e.g., https://play.min.io).
- `S3_REGION` — region for signing (default `us-east-1`).
- `S3_FORCE_PATH_STYLE` — set to `1` or `true` to force path-style addressing for S3 providers that require it.
- `OWNER_LOCK_MAX_MINS` — maximum owner hold duration in minutes for the S3 owner (default `10`).

Credentials follow standard AWS SDK environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, etc.).

Notes & next steps

- Ensure `restic` is installed in the runtime image and environment variables `RESTIC_REPOSITORY` and `RESTIC_PASSWORD` (and AWS credentials) are set for S3.
- Optionally set `RESTIC_AUTO_INIT=1` or `RESTIC_INIT_IF_MISSING=1` to automatically initialize the repository when it does not exist.

A browser-based UI is available when you start the binary with `--http-addr`, for example `--http-addr ":8080"`.

Example run

Build container:

```bash
docker build -t blt-volume-manager:local .
```

Run as plugin (binary mode on host):

```bash
sudo ./blt-volume-manager --data-dir /var/lib/docker-volumes --socket /run/docker/plugins/blt-volume-manager.sock
```

To run as a container, mount `/run/docker/plugins` and `/var/lib/docker-volumes` appropriately and set env vars for restic/S3.

Enable S3 owner (example build):

```bash
# implement S3 owner and build with tag 's3' OR vendor a package that provides it
go build -tags s3 -o blt-volume-manager ./...
```
