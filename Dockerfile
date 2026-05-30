FROM golang:1.25-alpine AS build
WORKDIR /src

# Install build dependencies
RUN apk add --no-cache git ca-certificates nodejs npm make

# Copy Go module files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build web UI (delegates to Makefile)
RUN make ui

# Build Go binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags='-s -w' -o /bin/blt-volume-manager

FROM scratch AS plugin
COPY --from=build /bin/blt-volume-manager /usr/local/bin/blt-volume-manager
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENTRYPOINT ["/usr/local/bin/blt-volume-manager"]

FROM scratch AS web
COPY --from=build /bin/blt-volume-manager /usr/local/bin/blt-volume-manager
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENTRYPOINT ["/usr/local/bin/blt-volume-manager", "--http-only"]