FROM golang:1.26-alpine AS build
WORKDIR /src

# Install build dependencies
RUN apk add --no-cache \
    git=2.52.0-r0 \
    ca-certificates=20260413-r0 \
    nodejs=24.14.1-r0 \
    npm=11.11.0-r0 \
    make=4.4.1-r3

# Copy Go module files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build web UI and Go binary
RUN make ui && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags='-s -w' -o /bin/blt-volume-manager

FROM scratch AS plugin
COPY --from=build /bin/blt-volume-manager /usr/local/bin/blt-volume-manager
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENTRYPOINT ["/usr/local/bin/blt-volume-manager"]

FROM scratch AS web
COPY --from=build /bin/blt-volume-manager /usr/local/bin/blt-volume-manager
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENTRYPOINT ["/usr/local/bin/blt-volume-manager", "--http-only"]