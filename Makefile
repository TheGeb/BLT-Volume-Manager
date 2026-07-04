# BLT Volume Manager - Makefile
# Usage: make [target]
# Override args: make dev-driver ARGS="--http-addr :9090"

.PHONY: all dev-driver dev-web build test lint lint-go format clean coverage check hadolint ui ui-dev-build run-driver tidy
.PHONY: build-driver build-web docker-driver docker-web

# Configurable run arguments (override from command line)
ARGS ?=

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo "unknown")

STATICCHECK ?= $(shell command -v staticcheck >/dev/null 2>&1 && echo staticcheck || echo 'GOTOOLCHAIN=go1.26.3 go run honnef.co/go/tools/cmd/staticcheck@latest')

LDFLAGS = -s -w \
	-X 'github.com/TheGeb/BLT-Volume-Manager/internal/app.Version=$(VERSION)' \
	-X 'github.com/TheGeb/BLT-Volume-Manager/internal/app.Commit=$(COMMIT)' \
	-X 'github.com/TheGeb/BLT-Volume-Manager/internal/app.Date=$(DATE)'

# Default target
all: lint build

# === Utility ===

format:
	golangci-lint fmt ./...

tidy:
	go mod tidy

clean:
	rm -f blt-volume-manager blt-volume-manager-web blt-volume-manager-plugin
	go clean -cache
	rm -rf web/ui/node_modules 2>/dev/null || true
	rm -f web/ui/node_modules/.install-stamp
	rm -rf internal/web/static/*

# === Lint ===

staticcheck:
	$(STATICCHECK) ./...

golangci-lint-check:
	golangci-lint fmt ./...
	golangci-lint run ./...

lint-go: format
	@$(MAKE) -j2 golangci-lint-check staticcheck

lint: format
	@$(MAKE) -j2 golangci-lint-check staticcheck
	cd web/ui && npm run lint

hadolint:
	@echo "--- Dockerfile lint ---"
	@DOCKER=$$(command -v docker 2>/dev/null || command -v podman 2>/dev/null); \
	if [ -z "$$DOCKER" ]; then \
		echo "Error: neither docker nor podman found" >&2; \
		exit 1; \
	fi; \
	$$DOCKER run --rm -i hadolint/hadolint < Dockerfile

# === Test ===

test:
	go test -race ./... -short
	cd web/ui && npm test

test-go:
	go test -race ./... -short

test-ui:
	cd web/ui && npm test

coverage:
	golangci-lint fmt ./...
	$(MAKE) staticcheck
	go test ./... -coverprofile=coverage.out -short
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# === Build ===

ui:
	cd web/ui && npm install
	cd web/ui && npm run build
	mkdir -p internal/web/static

run-web:
	go run ./cmd/web $(ARGS)

web/ui/node_modules/.install-stamp: web/ui/package.json web/ui/package-lock.json
	cd web/ui && npm install
	touch $@

ui-dev-build: web/ui/node_modules/.install-stamp
	cd web/ui && npx svelte-check
	cd web/ui && npm run lint:fix
	cd web/ui && npm run build
	mkdir -p internal/web/static

build-driver: tidy format
	go build -ldflags "$(LDFLAGS)" -o blt-volume-manager-plugin ./cmd/driver

build-web: tidy format ui
	go build -ldflags "$(LDFLAGS)" -o blt-volume-manager-web ./cmd/web

build: build-driver build-web

# === Docker ===

docker-web:
	docker build --target web -t blt-volume-manager-web:local .

docker-driver:
	docker build --target plugin -t blt-volume-manager-plugin:local .

# === Development ===

dev-web:
	@$(MAKE) -j3 lint-go hadolint ui-dev-build
	go run ./cmd/web $(ARGS)

# Independent checks (lint-go, hadolint, ui-dev-build) run in parallel with: make -j dev-driver
dev-driver: lint-go hadolint ui-dev-build
	go run ./cmd/driver $(ARGS)

# === CI ===

check: tidy lint-go coverage hadolint ui-dev-build
