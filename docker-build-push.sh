#!/usr/bin/env bash
set -euo pipefail

# Build and push multi-arch Docker images for both the plugin and web targets.
#
# Usage:
#   IMAGE=myrepo/blt-volume-manager VERSION=v1.2.3 ./docker-build-push.sh
#
# Defaults:
#   IMAGE   = blt-volume-manager
#   VERSION = $(git describe --tags --always --dirty)

IMAGE="${IMAGE:-blt-volume-manager}"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo latest)}"
PLATFORMS="${PLATFORMS:-linux/amd64,linux/arm64}"

BUILDER="blt-volume-manager-builder"

# Ensure the buildx builder exists
if ! docker buildx inspect "$BUILDER" >/dev/null 2>&1; then
  docker buildx create --name "$BUILDER" --driver docker-container --bootstrap
fi
docker buildx use "$BUILDER"

build_and_push() {
  local target="$1"
  local tag_suffix="$2"

  local tags=("${IMAGE}:${VERSION}${tag_suffix}" "${IMAGE}:latest${tag_suffix}")

  echo "--- Building ${target} for ${PLATFORMS}"
  docker buildx build \
    --target "$target" \
    --platform "$PLATFORMS" \
    $(printf ' --tag %s' "${tags[@]}") \
    --push \
    .
  echo "--- Pushed ${target}: ${tags[*]}"
}

build_and_push "plugin" ""
build_and_push "web" "-web"

echo "Done."
