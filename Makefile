# BLT Volume Manager - Makefile
# Usage: make [target]
# Override args: make dev ARGS="--http-only --http-addr :9090"

.PHONY: all dev build test lint clean

# Default target
all: lint build

# Configurable run arguments (override from command line)
ARGS ?=

# Development: full lint, build, and run
dev:
	cd web/ui && npm install
	cd web/ui && npx svelte-check
	cd web/ui && npm run lint:fix
	cd web/ui && npm run build
	mkdir -p internal/web/static
	cp -r web/static/* internal/web/static/
	golangci-lint run ./...
	go run . $(ARGS)

# Build the UI only
ui:
	cd web/ui && npm install
	cd web/ui && npm run build
	mkdir -p internal/web/static
	cp -r web/static/* internal/web/static/

# Build the Go binary (includes UI build)
build: ui
	go build -o blt-volume-manager ./...

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
lint:
	golangci-lint run ./...
	cd web/ui && npm run lint

# Clean build artifacts
clean:
	rm -f blt-volume-manager
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