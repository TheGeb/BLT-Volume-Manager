# Deferred: Fail-Fast Driver and Backend Initialization

## Status

Deferred. Do not implement as part of another task.

## Goal

Prevent the application from constructing a partially initialized driver when a required metadata backend cannot be opened.

## Intended change

- Change `internal/driver.New` to return `(*Driver, error)`.
- Return metadata backend initialization errors instead of logging and continuing.
- Update `cmd/driver/main.go` and all tests/callers.
- Decide explicitly whether metadata is mandatory or whether a supported metadata-disabled mode remains necessary.
- Ensure background goroutines are not started when construction fails.

## Scope ownership

This task owns constructor and startup call sites only:

- `internal/driver/driver.go`
- `cmd/driver/main.go`
- Direct constructor tests

Do not modify lock algorithms, version allocation, web cleanup, or frontend code.

## Verification

- `go test ./... -short`
- Add a test proving backend initialization failure is returned to the caller.
