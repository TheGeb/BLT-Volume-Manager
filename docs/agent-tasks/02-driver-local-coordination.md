# Driver Local Mount Coordination

## Goal

Prevent concurrent `Mount` calls in one driver process from acquiring duplicate owner state or starting duplicate backup schedules.

## Intended change

- Add per-volume coordination or an explicit per-volume state machine.
- Reserve lock acquisition before releasing synchronization, so a second mount cannot start the same acquisition.
- Keep network and backup operations outside the global mutex.
- Update `LockKey`, attachment counts, cancellation functions, and acquisition state under the appropriate lock.
- Preserve existing mount behavior and response format.

## Scope ownership

This task owns driver runtime state only:

- `internal/driver/api.go`
- Driver tests

Do not modify metadata lock algorithms, backend interfaces, web code, or frontend code.

## Verification

- Add a concurrent mount test for one volume.
- Verify only one owner-lock acquisition and one hot-backup schedule occur.
- Run `go test -race ./internal/driver/... -short`.
