# BLT Volume Manager

**Backups + Locks + Transferring** — A Docker volume plugin that uses restic to backup volumes to S3 and uses locking to coordinate access between multiple hosts.

Features

- Implements Docker volume plugin methods: Create, Remove, Mount, Unmount, Path, Get, List, Capabilities
- Uses a pluggable `locker` interface. The default is a simple file lock; replace with an S3-based locker (see https://github.com/Cool-fire/aws-s3-lock) for cross-host locks.
- Uses `restic` (external binary) to perform hot and cold backups on configurable schedules.

Restore selection (hot vs cold)

- Backups are tagged in restic as `hot` or `cold` (the manager uses `--tag`).
- When calling `Create` you can pass an option `restore=hot|cold|latest` to control which snapshot to restore on creation.
- The plugin currently restores `latest` by default if `restore` is not provided.

S3 locker (optional)

- To enable an S3-based cross-host locker, provide `S3_LOCK_BUCKET` (and optional `S3_LOCK_PREFIX`) environment variables.
- A stub is provided in `locker/s3_stub.go`. You can either build the project with a real S3 locker implementation behind the `s3` build tag, or implement `NewS3Locker` to wrap `github.com/Cool-fire/aws-s3-lock`.

- To enable an S3-based cross-host locker, provide `S3_LOCK_BUCKET` (and optional `S3_LOCK_PREFIX`) environment variables.
- This project includes an S3-compatible locker implementation in `locker/s3_locker.go` which uses the AWS SDK v2 but supports any S3 provider by setting `S3_ENDPOINT` and `S3_FORCE_PATH_STYLE` as needed.
- Lock keys are now stored under `volumes/<volume>/host-locks/` within the configured bucket/prefix, so you can share the same S3 bucket for both restic and lock metadata.

Environment variables used by the S3 locker:

- `S3_ENDPOINT` — optional custom S3 endpoint (e.g., https://play.min.io).
- `S3_REGION` — region for signing (default `us-east-1`).
- `S3_FORCE_PATH_STYLE` — set to `1` or `true` to force path-style addressing for S3 providers that require it.
- `S3_LOCK_MAX_MINS` — maximum lock hold duration in minutes for the S3 locker (default `10`).

Credentials follow standard AWS SDK environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, etc.).

Notes & next steps

- Ensure `restic` is installed in the runtime image and environment variables `RESTIC_REPOSITORY` and `RESTIC_PASSWORD` (and AWS credentials) are set for S3.
- Optionally set `RESTIC_AUTO_INIT=1` or `RESTIC_INIT_IF_MISSING=1` to automatically initialize the repository when it does not exist.

A browser-based UI is available when you start the binary with `--http-addr`, for example `--http-addr ":8080"`.

Example run

Build container:

```bash
docker build -t s3vol:local .
```

Run as plugin (binary mode on host):

```bash
sudo ./s3vol --data-dir /var/lib/docker-volumes --socket /run/docker/plugins/s3vol.sock
```

To run as a container, mount `/run/docker/plugins` and `/var/lib/docker-volumes` appropriately and set env vars for restic/S3.

Enable S3 locker (example build):

```bash
# implement S3 locker and build with tag 's3' OR vendor a package that provides it
go build -tags s3 -o s3vol ./...
```
