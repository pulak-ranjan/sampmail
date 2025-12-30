# SampMail Makefile
# Usage: make [target]

VERSION := $(shell cat VERSION 2>/dev/null || echo "0.0.0")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
CHANNEL ?= stable

LDFLAGS := -X github.com/pulak-ranjan/sampmail/internal/core.Version=$(VERSION)
LDFLAGS += -X github.com/pulak-ranjan/sampmail/internal/core.BuildTime=$(BUILD_TIME)
LDFLAGS += -X github.com/pulak-ranjan/sampmail/internal/core.GitCommit=$(GIT_COMMIT)
LDFLAGS += -X github.com/pulak-ranjan/sampmail/internal/core.Channel=$(CHANNEL)
LDFLAGS += -s -w

.PHONY: all build dev run clean test frontend release help

# Default target
all: build

# Build production binary
build:
	@echo "Building SampMail v$(VERSION)..."
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o sampmail ./cmd/server
	@echo "✓ Built ./sampmail"

# Build with race detector (development)
build-dev:
	go build -race -o sampmail ./cmd/server

# Run in development mode
dev:
	go run ./cmd/server

# Run the built binary
run: build
	./sampmail

# Build frontend
frontend:
	@echo "Building frontend..."
	cd web && npm install && npm run build
	@echo "✓ Frontend built"

# Run frontend dev server
frontend-dev:
	cd web && npm run dev

# Clean build artifacts
clean:
	rm -f sampmail
	rm -rf dist/
	rm -rf releases/
	rm -rf web/dist/

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Lint code
lint:
	golangci-lint run ./...

# Format code
fmt:
	go fmt ./...
	cd web && npm run format 2>/dev/null || true

# Build for multiple platforms
build-all:
	@chmod +x scripts/build.sh
	./scripts/build.sh

# Create release packages
package: build-all
	@echo "Creating release packages..."
	@mkdir -p releases
	@for f in dist/sampmail-*; do \
		if [ "$${f}" != "dist/sampmail-checksums.txt" ]; then \
			name=$$(basename $$f); \
			echo "Packaging $$name..."; \
			zip -j "releases/$${name}.zip" "$$f" README.md LICENSE.md 2>/dev/null || true; \
		fi \
	done
	@echo "✓ Packages in releases/"

# Full release process
release:
	@if [ -z "$(v)" ]; then \
		echo "Usage: make release v=0.1.12"; \
		exit 1; \
	fi
	@chmod +x scripts/release.sh
	./scripts/release.sh $(v)

# Show current version
version:
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Channel: $(CHANNEL)"

# Database migrations
migrate:
	./sampmail migrate

# Install dependencies
deps:
	go mod download
	go mod tidy
	cd web && npm install

# Docker build
docker:
	docker build -t sampmail:$(VERSION) .

# Docker compose up
docker-up:
	docker-compose up -d

# Docker compose down
docker-down:
	docker-compose down

# Install systemd service
install-service:
	@echo "Installing systemd service..."
	sudo cp scripts/sampmail.service /etc/systemd/system/
	sudo systemctl daemon-reload
	sudo systemctl enable sampmail
	@echo "✓ Service installed. Start with: sudo systemctl start sampmail"

# Show help
help:
	@echo "SampMail v$(VERSION) - Build Targets"
	@echo ""
	@echo "Development:"
	@echo "  make build        - Build production binary"
	@echo "  make dev          - Run in development mode"
	@echo "  make run          - Build and run"
	@echo "  make test         - Run tests"
	@echo "  make lint         - Lint code"
	@echo "  make fmt          - Format code"
	@echo ""
	@echo "Frontend:"
	@echo "  make frontend     - Build frontend"
	@echo "  make frontend-dev - Run frontend dev server"
	@echo ""
	@echo "Release:"
	@echo "  make build-all    - Build for all platforms"
	@echo "  make package      - Create release packages"
	@echo "  make release v=X.Y.Z - Full release process"
	@echo ""
	@echo "Docker:"
	@echo "  make docker       - Build Docker image"
	@echo "  make docker-up    - Start with docker-compose"
	@echo "  make docker-down  - Stop docker-compose"
	@echo ""
	@echo "Other:"
	@echo "  make clean        - Remove build artifacts"
	@echo "  make deps         - Install dependencies"
	@echo "  make version      - Show version info"
	@echo "  make install-service - Install systemd service"
