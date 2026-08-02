ARG VERSION=dev
ARG COMMIT=unknown

FROM --platform=$BUILDPLATFORM golang:1.26-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS build
ARG VERSION
ARG COMMIT
ARG TARGETARCH
WORKDIR /src

RUN apk add --no-cache \
    git=2.54.0-r0 \
    ca-certificates=20260611-r0 \
    nodejs=24.18.1-r0 \
    npm=11.12.1-r0 \
    make=4.4.1-r4

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY web/ui/package.json web/ui/package-lock.json ./web/ui/
RUN --mount=type=cache,target=/root/.npm \
    npm ci --prefix web/ui

COPY . .

WORKDIR /src/web/ui
RUN npm run build

WORKDIR /src
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    mkdir -p internal/web/static && \
    cp -r web/ui/dist/* internal/web/static/ && \
    CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build -ldflags "-s -w \
      -X github.com/TheGeb/BLT-Volume-Manager/internal/app.Version=$VERSION \
      -X github.com/TheGeb/BLT-Volume-Manager/internal/app.Commit=$COMMIT \
      -X github.com/TheGeb/BLT-Volume-Manager/internal/app.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o blt-volume-manager-plugin ./cmd/driver && \
    CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build -ldflags "-s -w \
      -X github.com/TheGeb/BLT-Volume-Manager/internal/app.Version=$VERSION \
      -X github.com/TheGeb/BLT-Volume-Manager/internal/app.Commit=$COMMIT \
      -X github.com/TheGeb/BLT-Volume-Manager/internal/app.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o blt-volume-manager-web ./cmd/web

# Runtime base — provides a shared non-root user for targets that support it
FROM restic/restic:0.19.1@sha256:136600b6ff6843d61d355f7f71f460a166429f35de6fd11b568fece3c9a4d510 AS base
RUN adduser -D -u 1001 appuser
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
USER appuser
ENTRYPOINT ["/usr/local/bin/blt-volume-manager"]

# Plugin — requires root for Docker socket and volume mount access
FROM base AS plugin
# The plugin needs root for Docker socket and volume mount access.
# hadolint ignore=DL3002
USER root
COPY --from=build /src/blt-volume-manager-plugin /usr/local/bin/blt-volume-manager

# Web — can run as non-root
FROM base AS web
COPY --from=build /src/blt-volume-manager-web /usr/local/bin/blt-volume-manager
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 CMD ["/usr/local/bin/blt-volume-manager", "--health"]
