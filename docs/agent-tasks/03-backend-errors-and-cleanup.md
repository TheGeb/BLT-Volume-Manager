# Backend Error Classification and Cleanup Propagation

## Goal

Preserve meaningful backend failures and prevent delete, rename, or copy operations from reporting success after incomplete cleanup.

## Intended change

- In owner validation, return `(false, nil)` only for `ErrKeyNotFound`; propagate S3/etcd outages and other read errors.
- Make repository existence checks distinguish a known missing repository from authentication, corruption, command, and network failures.
- Make restic repository initialization return errors, ignoring only a specifically recognized already-initialized result.
- Inspect per-object errors in S3 batch deletion responses.
- Make `CleanupVolumeData` return an error and propagate it through delete and rename handlers.
- Ensure target repositories created by failed copy/rename operations are cleaned up or left in an explicitly recoverable state.

## Scope ownership

This task owns:

- `internal/restic/repo.go`
- `internal/restic/backup.go`
- `internal/s3/client.go`
- `internal/web/volume/volume.go`
- `internal/web/volume/handler.go`
- Focused tests for these behaviors

Do not modify metadata owner-lock error classification; that file belongs to the metadata atomicity task. Do not introduce a general typed-error package here; preserve existing exported APIs unless necessary for error propagation. The typed API error work is separate.

## Verification

- Test backend outage versus missing-key behavior.
- Test partial S3 deletion failure.
- Test failed cleanup produces a failed HTTP response.
- `go test ./internal/restic/... ./internal/s3/... ./internal/web/volume/... -short`
