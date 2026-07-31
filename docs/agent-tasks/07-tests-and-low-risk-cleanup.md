# Tests and Low-Risk Structural Cleanup

## Goal

Improve test diagnostics and complete behavior-preserving cleanup that is not part of another feature task.

## Intended change

- Expand `internal/web/server/server_test.go` for implicit writes, duplicate status writes, gzip negotiation, `Vary`, `NoSniff`, and response handling.
- Make integration helpers fail on JSON decode errors in `test/integration/web_test.go`.
- Replace unchecked integration-test type assertions with typed response structs or assertion helpers.
- Remove unreachable statements after `t.Fatal` in driver tests.
- Remove `t.Parallel()` from integration tests that share mutable repositories, volumes, or servers, or isolate their fixtures.
- Add reduced-motion CSS and loading status semantics only if not already covered by the accessibility task; otherwise leave those changes to the accessibility task.

## Scope ownership

This task owns tests and test helpers only. Tests for production files modified by another task belong to that task, so this task must not edit those same test files.

Do not modify production behavior already owned by the other task files.

## Verification

- `go test -race ./... -short`
- `npm test`
- `npm run check`
