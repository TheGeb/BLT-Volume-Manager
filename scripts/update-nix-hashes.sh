#!/usr/bin/env bash
set -euo pipefail

# update-nix-hashes.sh — Update vendorHash or npmDepsHash in flake.nix
#
# Usage:
#   scripts/update-nix-hashes.sh vendor   # update vendorHash
#   scripts/update-nix-hashes.sh npm      # update npmDepsHash

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
FLAKE="$REPO_ROOT/flake.nix"

if [ ! -f "$FLAKE" ]; then
  echo "ERROR: flake.nix not found at $FLAKE" >&2
  exit 1
fi

case "${1:-}" in
  vendor)
    HASH_VAR="vendorHash"
    BUILD_ATTR="blt-volume-manager"
    ;;
  npm)
    HASH_VAR="npmDepsHash"
    BUILD_ATTR="ui"
    ;;
  *)
    echo "Usage: $0 {vendor|npm}" >&2
    exit 1
    ;;
esac

# Build from a complete temporary repository so relative flake inputs remain
# available while the hash is intentionally blanked.
TMPDIR="$(mktemp -d)"
TMPREPO="$TMPDIR/repo"
cp -a "$REPO_ROOT/." "$TMPREPO"
trap 'rm -rf "$TMPDIR"' EXIT

# Extract old hash
OLD_HASH="$(grep -oP "${HASH_VAR} = \"\K[^\"]+" "$FLAKE" || true)"
echo "Current $HASH_VAR: ${OLD_HASH:-<empty>}"

# Clear the hash so Nix computes it
sed -i "s/${HASH_VAR} = \"[^\"]*\"/${HASH_VAR} = \"\"/" "$TMPREPO/flake.nix"

# Build to get the computed hash
echo "Building ${BUILD_ATTR} to compute hash..."
set +e
BUILD_OUTPUT="$(nix build "$TMPREPO#$BUILD_ATTR" 2>&1)"
BUILD_STATUS=$?
set -e
mapfile -t HASHES < <(printf '%s\n' "$BUILD_OUTPUT" | grep -oP 'got:\s*\K\S+' | sort -u || true)

if [ "$BUILD_STATUS" -eq 0 ] || [ "${#HASHES[@]}" -ne 1 ]; then
  echo "ERROR: Expected one hash mismatch from the Nix build" >&2
  echo "Build output:" >&2
  echo "$BUILD_OUTPUT" >&2
  exit 1
fi
NEW_HASH="${HASHES[0]}"

if [ "$NEW_HASH" = "$OLD_HASH" ]; then
  echo "$HASH_VAR is up to date ($OLD_HASH)"
else
  # Update the real flake.nix with the new hash
  sed -i "s|${HASH_VAR} = \"[^\"]*\"|${HASH_VAR} = \"${NEW_HASH}\"|" "$FLAKE"
  echo "$HASH_VAR: ${OLD_HASH:-<empty>} → $NEW_HASH"
  echo "Updated $HASH_VAR in flake.nix"
fi
