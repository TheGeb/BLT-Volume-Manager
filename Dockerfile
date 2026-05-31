FROM golang:1.26-alpine AS build
WORKDIR /src

RUN apk add --no-cache \
    git=2.52.0-r0 \
    ca-certificates=20260413-r0 \
    nodejs=24.14.1-r0 \
    npm=11.11.0-r0 \
    make=4.4.1-r3

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN make ui && \
    CGO_ENABLED=0 GOOS=linux make build

FROM scratch AS plugin
COPY --from=build /src/blt-volume-manager-plugin /usr/local/bin/blt-volume-manager
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENTRYPOINT ["/usr/local/bin/blt-volume-manager"]

FROM alpine:3.22 AS web
COPY --from=build /src/blt-volume-manager-web /usr/local/bin/blt-volume-manager
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 CMD wget -qO- http://localhost:8080/api/health || exit 1
ENTRYPOINT ["/usr/local/bin/blt-volume-manager"]
