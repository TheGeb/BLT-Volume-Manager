# Metadata Migration (etcd ↔ S3)

BLT Volume Manager stores its **metadata** (owner locks, registered volumes,
version counters, restore points) in one of two backends - S3 or etcd. The
migration tool copies that metadata between backends so you can switch your
deployment from one to the other. The **restic backup data** itself is never
touched; it lives in `RESTIC_REPOSITORY` and is independent of the metadata
backend.

The migration logic is built into the **web binary** (`cmd/web`), so it is
available both as a host binary and inside the web container image.

## What gets migrated

- **Owner locks** - migrated per volume (the currently winning lock), with a
  **refreshed** expiry:
  - permanent locks stay permanent;
  - time-limited locks are re-anchored to the export time (`now + remaining
    TTL`), so an active lock remains valid in the target without trying to
    preserve the exact remaining seconds;
  - already-expired locks are skipped.

  This rule is the **same for both backends**; the mechanism differs only
  because each backend enforces a lock's lifetime differently:
  - **S3** encodes lifetime as an absolute wall-clock timestamp on the
    proposal object, which readers stop honoring once it passes.
  - **etcd** enforces lifetime with a **lease**: the lock key is deleted when
    its lease expires. So a migrated time-limited lock gets a lease covering
    the remaining TTL and is deliberately **not** keep-alive tracked, so it
    self-expires once that time elapses. (Keepalive would pin the lock to the
    short-lived migration process that doesn't own the volume; the real owner
    restarts against the new backend as part of the cutover.)
  - A **permanent lock** migrated to etcd is written **lease-less**, so it
    never expires and stays until removed.

  If the target already holds a lock for a volume, the migration refreshes it
  when it is the same owner and **aborts with an error** when a different
  owner holds it - for both backends - so it never silently steals an active
  lock.

  **Lease ownership caveat:** a migrated etcd lock is held by the migration
  process's lease grant, not by the future host process. Because release is
  tracked per-process, hosts cannot `ReleaseLock` an imported lock through the
  normal path. Time-limited locks solve this by expiring on their own. Imported
  locks are additionally **flagged as migrated** in the target store, so the
  next host that mounts the volume **takes the lock over** (release-then-own)
  instead of needlessly leaving a process-bound lease behind; a lock still
  actively held by a different live owner is never stolen - takeover only ever
  applies to a lock the migration wrote for a volume that has no active host.
- **Registered volumes**, **version counters**, and **restore points** - copied
  verbatim.

Because the tool migrates the *winner* per volume, a partial re-run on a target
that already received some locks will pick the oldest valid proposal, which
matches the S3 algorithm's behavior.

## Running

The migration is available two ways - from the **web UI** and as a
**subcommand** of the web binary (which is what runs in the web image).

### From the web UI

The migration applies to the **entire metadata database**, not a single volume.
Click **Migrate metadata** in the top bar (next to Refresh) - it is available on
every page, with or without a volume selected. Configure the source and
destination backends (each side supports "Use current backend" to prefill this
server's configured backend), then **Dry run** to preview, or **Run migration**.
S3 credentials always come from this server's environment, never from the
browser.

### CLI subcommand

```bash
# host build
go build ./cmd/web
./blt-volume-manager-web migrate --from-type etcd --to-type s3 ...

# web container image (entrypoint is the web binary)
docker run --rm \
  -e AWS_ACCESS_KEY_ID=... -e AWS_SECRET_ACCESS_KEY=... \
  ghcr.io/thegeb/blt-volume-manager-web:latest \
  migrate --from-type etcd --from-etcd-endpoints http://etcd:2379 \
          --to-type s3 --to-bucket my-meta --to-endpoint https://s3.example.com
```

## Flags

Every flag exists for both sides (`--from-*` and `--to-*`):

| Flag | Meaning |
|---|---|
| `--from-type s3\|etcd` / `--to-type s3\|etcd` | Metadata backend (required) |
| `--from-bucket` / `--to-bucket` | S3 bucket (for `type=s3`) |
| `--from-endpoint` / `--to-endpoint` | S3 endpoint, e.g. `https://minio:9000` |
| `--from-region` / `--to-region` | S3 region (for signing) |
| `--from-force-path-style` / `--to-force-path-style` | Use path-style S3 URLs; **default `true`** (required for MinIO, Garage, etc.) |
| `--from-etcd-endpoints` / `--to-etcd-endpoints` | Comma-separated etcd endpoints (for `type=etcd`) |
| `--dry-run` | Print what would be migrated without writing anything |

S3 credentials come from the standard AWS environment variables
(`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_DEFAULT_REGION`). See the
README's [Configuration section](../README.md#configuration).

## Examples

Preview an etcd → S3 migration (writes nothing):

```bash
./blt-volume-manager-web migrate \
  --from-type etcd --from-etcd-endpoints http://127.0.0.1:2379 \
  --to-type s3 --to-bucket my-meta --to-endpoint https://s3.example.com \
  --dry-run
```

Run it for real:

```bash
./blt-volume-manager-web migrate \
  --from-type etcd --from-etcd-endpoints http://127.0.0.1:2379 \
  --to-type s3 --to-bucket my-meta --to-endpoint https://s3.example.com --to-region us-east-1
```

S3 → etcd:

```bash
./blt-volume-manager-web migrate \
  --from-type s3 --from-bucket my-meta --from-endpoint https://s3.example.com \
  --to-type etcd --to-etcd-endpoints http://127.0.0.1:2379,http://etcd2:2379
```

Both sides can even be S3 (e.g. moving between buckets) or etcd (moving between
clusters):

```bash
./blt-volume-manager-web migrate \
  --from-type s3 --from-bucket old-meta --from-endpoint https://s3.example.com \
  --to-type s3 --to-bucket new-meta --to-endpoint https://s3.example.com
```

## Before you run

- **Run a `--dry-run` first** and review the output.
- **No volumes may be mounted during the migration.** Stop the plugin/web
  daemons (or at least ensure no host is actively mounting a volume) so no new
  owner locks are taken or refreshed mid-cutover. Migrating owner locks while a
  host holds one can split or drop ownership; a mounted host must not be
  running against two backends at once.
- **Stop the plugin/web daemons** (or at least point every host at one backend)
  so no new owner locks or version counters are written while the copy is in
  flight. Version counters are not atomic across independent writers without a
  coordinator, so migrating a live system can lose increments.
- **Active etcd locks** are lease-based and kept alive by the process holding
  them. The tool re-anchors their expiry so the target sees a valid lock, but
  the original process still holds its lease on the old backend until it is
  restarted against the new one. Restart hosts against the target backend as
  part of the cutover.
- After migration, point all hosts at the new backend (e.g. `BLT_METADATA_BACKEND`
  + `METADATA_S3_BUCKET`/`ETCD_ENDPOINTS`) and verify volumes, owners, and
  snapshots look correct before retiring the old backend.

## Backward direction

Both directions are supported by swapping `--from-*` / `--to-*`. The S3 backend
tracks locks with timestamped proposal keys and best-effort compare-and-list;
the etcd backend uses atomic CAS with leases. Migrating **from** S3 drops the
extra proposal keys (only the winner is written to etcd). Migrating **from**
etcd writes the single lock key into S3 in the proposal format.
