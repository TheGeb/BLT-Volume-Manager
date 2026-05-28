#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
EXIT_CODE=0

echo "========================================"
echo "  Running all tests (Go + TypeScript)"
echo "========================================"
echo

# ---- Go tests ----
echo "--- Go unit tests ---"
if go test ./... -count=1 "$@"; then
    echo -e "\n✓ Go unit tests passed"
else
    echo -e "\n✗ Go unit tests failed" >&2
    EXIT_CODE=1
fi

echo
echo "--- Go integration tests (requires Docker) ---"
if docker info &>/dev/null; then
    if go test -tags=integration -count=1 -v "$@" .; then
        echo -e "\n✓ Go integration tests passed"
    else
        echo -e "\n✗ Go integration tests failed" >&2
        EXIT_CODE=1
    fi
else
    echo "  (skipped - Docker not available)"
fi

echo

# ---- TypeScript tests ----
echo "--- TypeScript tests ---"
if cd "$ROOT/web/ui" && npm test -- "$@"; then
    echo -e "\n✓ TypeScript tests passed"
else
    echo -e "\n✗ TypeScript tests failed" >&2
    EXIT_CODE=1
fi

echo
echo "========================================"
if [ "$EXIT_CODE" -eq 0 ]; then
    echo "  All tests passed!"
else
    echo "  Some tests failed (exit code $EXIT_CODE)" >&2
fi
echo "========================================"

exit "$EXIT_CODE"
