# Metadata Lock and Version Atomicity

## Goal

Prevent duplicate distributed ownership and duplicate version allocations without using S3-native conditional features that Garage may not support.

## Backend rules

- Do not use `If-None-Match`, conditional object creation, or other S3-specific atomic features.
- S3-only ownership remains best-effort because generic S3 operations cannot provide a strict distributed compare-and-swap lock.
- When etcd is configured, use etcd as the coordination backend for ownership and version allocation.
- Use an etcd transaction/CAS operation for acquisition and a lease for ownership TTL.
- Renew the etcd lease while ownership is active and release it on normal shutdown.
- Do not use etcd revisions as wall-clock timestamps.

## Owner expiration and ordering

- Preserve the S3 algorithm's existing expiry check based on the expiry encoded in the owner key.
- For etcd, use the native lease TTL as the expiration mechanism; do not run the S3 wall-clock stale-object check against etcd revisions.
- `ModificationCounter` may continue to represent an ordering value for choosing the earliest proposal where that is appropriate.
- If the shared object model is changed, make the semantic distinction explicit without converting etcd revisions into timestamps.

## Version allocation

- Replace `ReadCounter` plus `WriteCounter` with an atomic etcd transaction/CAS operation when etcd coordinates metadata.
- Preserve the existing major/minor version semantics and returned tag format.
- If S3-only mode remains supported, document that version allocation is not safe across independent writers unless another coordinator is configured. Do not invent an unsupported S3 atomic primitive.

## Scope ownership

This task owns metadata coordination and version code only:

- `internal/metadata/store/owner.go`
- `internal/metadata/store/version.go`
- `internal/metadata/store/store.go` and backend interfaces if required
- `internal/metadata/etcd/etcd.go`
- Relevant metadata tests

This task also owns owner-lock error classification in `owner.go`, including preserving the distinction between `ErrKeyNotFound` and backend outages.

Do not modify `internal/driver/api.go`, web handlers, or frontend files. A separate task owns local driver synchronization.

## Verification

- Add concurrent etcd tests for lock acquisition, lease expiration, renewal, and version allocation.
- Add S3 tests confirming the existing S3 expiry behavior remains intact.
- `go test -race ./internal/metadata/... -short`
