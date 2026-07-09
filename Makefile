# BLT Volume Manager - Makefile
# Usage: make [target]
# Override args: make dev-driver ARGS="--http-addr :9090"

.PHONY: all dev-driver dev-web build test lint lint-go format clean coverage check hadolint ui ui-dev-build run-driver tidy
.PHONY: nix-vendor-hash nix-npm-hash nix-hashes
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

# === Nix hashes ===

nix-vendor-hash:
	@OLD=$$(grep -oP 'vendorHash = "\K[^"]+' flake.nix); \
	cp flake.nix flake.nix.bak; \
	sed -i 's/vendorHash = "[^"]*"/vendorHash = ""/' flake.nix; \
	OUTPUT=$$(nix build '.#blt-volume-manager' 2>&1 || true); \
	NEW=$$(echo "$$OUTPUT" | grep -oP 'got:\s*\K\S+'); \
	if [ -n "$$NEW" ] && [ "$$NEW" != "$$OLD" ]; then \
		sed -i "s|vendorHash = \"\"|vendorHash = \"$$NEW\"|" flake.nix; \
		echo "vendorHash: $$OLD → $$NEW"; \
		echo "Updated vendorHash in flake.nix"; \
	elif [ -n "$$NEW" ]; then \
		sed -i "s|vendorHash = \"\"|vendorHash = \"$$OLD\"|" flake.nix; \
		echo "vendorHash is up to date ($$OLD)"; \
	else \
		mv flake.nix.bak flake.nix; \
		echo "Failed to extract vendorHash — restored original" >&2; \
		exit 1; \
	fi; \
	rm -f flake.nix.bak

nix-npm-hash:
	@OLD=$$(grep -oP 'npmDepsHash = "\K[^"]+' flake.nix); \
	cp flake.nix flake.nix.bak; \
	sed -i 's/npmDepsHash = "[^"]*"/npmDepsHash = ""/' flake.nix; \
	OUTPUT=$$(nix build '.#ui' 2>&1 || true); \
	NEW=$$(echo "$$OUTPUT" | grep -oP 'got:\s*\K\S+'); \
	if [ -n "$$NEW" ] && [ "$$NEW" != "$$OLD" ]; then \
		sed -i "s|npmDepsHash = \"\"|npmDepsHash = \"$$NEW\"|" flake.nix; \
		echo "npmDepsHash: $$OLD → $$NEW"; \
		echo "Updated npmDepsHash in flake.nix"; \
	elif [ -n "$$NEW" ]; then \
		sed -i "s|npmDepsHash = \"\"|npmDepsHash = \"$$OLD\"|" flake.nix; \
		echo "npmDepsHash is up to date ($$OLD)"; \
	else \
		mv flake.nix.bak flake.nix; \
		echo "Failed to extract npmDepsHash — restored original" >&2; \
		exit 1; \
	fi; \
	rm -f flake.nix.bak

nix-hashes: nix-vendor-hash nix-npm-hash

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
	cp -r web/ui/dist/* internal/web/static/

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

build-driver:
	go build -ldflags "$(LDFLAGS)" -o blt-volume-manager-plugin ./cmd/driver

build-web: ui
	go build -ldflags "$(LDFLAGS)" -o blt-volume-manager-web ./cmd/web

build: build-driver build-web

# Use in Docker/release builds where golangci-lint isn't available
build-release:
	go build -ldflags "$(LDFLAGS)" -o blt-volume-manager-plugin ./cmd/driver
	go build -ldflags "$(LDFLAGS)" -o blt-volume-manager-web ./cmd/web

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
