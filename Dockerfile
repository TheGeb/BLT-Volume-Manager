ARG VERSION=dev
ARG COMMIT=unknown

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG VERSION
ARG COMMIT
ARG TARGETARCH
WORKDIR /src

RUN apk add --no-cache \
    git=2.54.0-r0 \
    ca-certificates=20260611-r0 \
    nodejs=24.17.0-r0 \
    npm=11.12.1-r0 \
    make=4.4.1-r4

COPY go.mod go.sum ./
RUN go mod download

# Cache npm dependencies separately from source code
COPY web/ui/package.json web/ui/package-lock.json ./web/ui/
RUN npm ci --prefix web/ui

COPY . .

WORKDIR /src/web/ui
RUN npm run build

WORKDIR /src
RUN --mount=type=cache,target=/root/.cache/go-build \
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

FROM scratch AS plugin
COPY --from=build /src/blt-volume-manager-plugin /usr/local/bin/blt-volume-manager
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENTRYPOINT ["/usr/local/bin/blt-volume-manager"]

FROM alpine:3.24 AS web
COPY --from=build /src/blt-volume-manager-web /usr/local/bin/blt-volume-manager
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 CMD wget -qO- http://localhost:8080/api/health || exit 1
ENTRYPOINT ["/usr/local/bin/blt-volume-manager"]
