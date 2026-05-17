FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN apk add --no-cache git ca-certificates
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags='-s -w' -o /bin/s3vol

FROM scratch
COPY --from=build /bin/s3vol /usr/local/bin/s3vol
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENTRYPOINT ["/usr/local/bin/s3vol"]
