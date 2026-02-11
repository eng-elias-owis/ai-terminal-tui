# Makefile for AI Terminal TUI
# Supports Linux, Windows, and macOS

# Variables
BINARY_NAME := ai-terminal-tui
MODULE := github.com/eng-elias-owis/ai-terminal-tui
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S' 2>/dev/null || echo "unknown")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOINSTALL := $(GOCMD) install
GOMOD := $(GOCMD) mod

# Platforms
PLATFORMS := linux/amd64 linux/arm64 windows/amd64 darwin/amd64 darwin/arm64

.PHONY: all build clean test dev install build-linux build-windows build-darwin help air

# Default target
all: build

## help: Show this help message
help:
	@echo "Available targets:"
	@awk '/^## /{gsub(/^## /,"",$$0); print "  "$$0}' $(MAKEFILE_LIST) | column -t -s ':' || true
	@echo ""
	@echo "Standard targets:"
	@echo "  make build         - Build for current platform"
	@echo "  make build-linux   - Build for Linux"
	@echo "  make build-windows - Build for Windows"
	@echo "  make dev           - Run with auto-reload using air"
	@echo "  make install       - Install via go install"
	@echo "  make test          - Run tests"
	@echo "  make clean         - Clean build artifacts"

## dev: Run with auto-reload using air
dev:
	@echo "Checking for air..."
	@which air > /dev/null || (echo "Installing air..." && go install github.com/air-verse/air@latest)
	@echo "Starting development server with auto-reload..."
	air

## build: Build for current platform
build:
	@echo "Building $(BINARY_NAME) for current platform..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) .
	@echo "Build complete: $(BINARY_NAME)"

## build-linux: Build for Linux (amd64 and arm64)
build-linux:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64 .
	@echo "Linux builds complete in bin/"

## build-windows: Build for Windows (amd64)
build-windows:
	@echo "Building for Windows..."
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe .
	@echo "Windows build complete in bin/"

## build-darwin: Build for macOS (amd64 and arm64)
build-darwin:
	@echo "Building for macOS..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 .
	@echo "macOS builds complete in bin/"

## build-all: Build for all platforms
build-all: build-linux build-windows build-darwin
	@echo "All platform builds complete in bin/"

## install: Install via go install
install:
	@echo "Installing $(MODULE)@latest..."
	$(GOINSTALL) $(LDFLAGS) $(MODULE)@latest
	@echo "Installation complete!"

## test: Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

## clean: Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -rf bin/
	rm -rf tmp/
	@echo "Clean complete"

## tidy: Tidy and verify go modules
tidy:
	@echo "Tidying go modules..."
	$(GOMOD) tidy
	$(GOMOD) verify

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./...

## fmt: Format Go code
fmt:
	@echo "Formatting Go code..."
	$(GOCMD) fmt ./...

## air-install: Install air live-reload tool
air-install:
	@echo "Installing air..."
	$(GOINSTALL) github.com/air-verse/air@latest

# Cross-compilation targets for specific platforms
.PHONY: linux-amd64 linux-arm64 windows-amd64 darwin-amd64 darwin-arm64

linux-amd64:
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 .

linux-arm64:
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64 .

windows-amd64:
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe .

darwin-amd64:
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 .

darwin-arm64:
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 .
