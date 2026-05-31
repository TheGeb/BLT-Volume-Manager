# BLT Volume Manager - Makefile
# Usage: make [target]
# Override args: make dev ARGS="--http-only --http-addr :9090"

.PHONY: all dev build test lint lint-go format clean coverage check hadolint ui ui-dev run watch tidy

# Default target
all: lint build

# Configurable run arguments (override from command line)
ARGS ?=

# Format Go code automatically
format:
	golangci-lint fmt ./...

# Go lint (format first for consistency, then lint)
lint-go:
	golangci-lint fmt ./...
	golangci-lint run ./...

# Development: full lint, build, and run
# Independent checks (lint-go, hadolint, ui-dev) run in parallel with: make -j dev
dev: lint-go hadolint ui-dev
	go run . $(ARGS)

dev-ui: ui-dev
	cd web/ui && npm run dev

# CI / full check (includes tests and coverage)
check: tidy lint-go coverage hadolint ui-dev

# Go test coverage report
coverage:
	golangci-lint fmt ./...
	go test ./... -coverprofile=coverage.out -short
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Dockerfile lint via Docker (falls back to podman)
hadolint:
	@echo "--- Dockerfile lint ---"
	@DOCKER=$$(command -v docker 2>/dev/null || command -v podman 2>/dev/null); \
	if [ -z "$$DOCKER" ]; then \
		echo "Error: neither docker nor podman found" >&2; \
		exit 1; \
	fi; \
	$$DOCKER run --rm -i hadolint/hadolint < Dockerfile

# Build the UI only
ui:
	cd web/ui && npm install
	cd web/ui && npm run build
	mkdir -p internal/web/static
	cp -r web/static/* internal/web/static/

# UI development build (includes typecheck and lint:fix)
ui-dev:
	cd web/ui && npm install
	cd web/ui && npm run check
	cd web/ui && npm run lint:fix
	cd web/ui && npm run build
	mkdir -p internal/web/static
	cp -r web/static/* internal/web/static/

# Tidy Go module dependencies
tidy:
	go mod tidy

# Build the Go binary (includes UI build)
build: tidy format ui
	go build -o blt-volume-manager .

# Run all tests
test:
	go test ./... -short
	cd web/ui && npm test

# Run Go tests only
test-go:
	go test ./... -short

# Run UI tests only
test-ui:
	cd web/ui && npm test

# Run linting
lint: format
	golangci-lint run ./...
	cd web/ui && npm run lint

# Clean build artifacts
clean:
	rm -f blt-volume-manager
	go clean -cache
	rm -rf web/ui/node_modules
	rm -rf web/static/*
	rm -rf internal/web/static/*

# Docker build
docker:
	docker build --target web -t blt-volume-manager:local .

# Quick run (assumes UI is already built)
run:
	go run . $(ARGS)

# Watch mode for development (requires entr or similar)
watch:
	@echo "Install entr: sudo apt install entr"
	find . -name '*.go' | entr -r go run . $(ARGS)