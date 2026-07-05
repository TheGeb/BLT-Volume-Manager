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

Configuration

Copy the following into a `.env` file and adjust for your setup:

```env
# --- Required ---
# Restic repository URL (S3 or local path)
RESTIC_REPOSITORY=s3:http://your-s3-endpoint:3900/your-bucket

# Restic repository password
RESTIC_PASSWORD=changeme

# --- S3 Owner (optional cross-host ownership) ---
# S3 endpoint for the owner bucket (auto-derived from RESTIC_REPOSITORY if unset)
# S3_ENDPOINT=http://your-s3-endpoint:3900

# S3 region for signing
# S3_REGION=us-east-1

# Dedicated S3 bucket for owner locks (defaults to the restic bucket if unset)
# OWNER_LOCK_S3_BUCKET=your-owner-bucket

# Maximum owner hold duration in minutes (default: 10)
# OWNER_MAX_MINS=10

# Force path-style S3 addressing (set to 1 or true for MinIO, Garage, etc.)
# S3_FORCE_PATH_STYLE=1

# --- AWS Credentials ---
# AWS_ACCESS_KEY_ID=your-access-key
# AWS_SECRET_ACCESS_KEY=your-secret-key
# AWS_DEFAULT_REGION=us-east-1

# --- Optional ---
# Enable dummy/test volume creation via the web UI
# BLT_TEST_MODE=1

# Log level: debug, info, warn, error (default: info)
# LOG_LEVEL=info
```

S3 owner (optional)

- To enable an S3-based cross-host owner, provide `OWNER_LOCK_S3_BUCKET` environment variable.
- Owner keys are stored under `blt-volume-manager/owners/<volume>/` within the configured bucket.
- Credentials follow standard AWS SDK environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, etc.).

Next steps

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
