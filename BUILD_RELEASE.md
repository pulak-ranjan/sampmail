# SampMail Build & Release Guide

This guide explains how to build SampMail, create releases, and publish to GitHub for the auto-update system.

## Prerequisites

```bash
# Go 1.21+
go version

# Node.js 18+
node --version

# Git
git --version

# GitHub CLI (optional but recommended)
gh --version
```

---

## 1. Local Development Build

### Quick Build (Development)

```bash
cd /opt/sampmail

# Build Go binary
go build -o sampmail ./cmd/server

# Build frontend
cd web && npm install && npm run build && cd ..

# Run
./sampmail
```

### Production Build with Version Info

```bash
# Set version variables
VERSION=$(cat VERSION)
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(git rev-parse --short HEAD)
GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD)

# Build with ldflags
go build -ldflags "\
  -X github.com/pulak-ranjan/sampmail/internal/core.Version=${VERSION} \
  -X github.com/pulak-ranjan/sampmail/internal/core.BuildTime=${BUILD_TIME} \
  -X github.com/pulak-ranjan/sampmail/internal/core.GitCommit=${GIT_COMMIT} \
  -X github.com/pulak-ranjan/sampmail/internal/core.Channel=stable \
  -s -w" \
  -o sampmail ./cmd/server

# Verify version
./sampmail --version
```

---

## 2. Build for Multiple Platforms

### Build Script (build.sh)

Create `scripts/build.sh`:

```bash
#!/bin/bash
set -e

VERSION=$(cat VERSION)
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
CHANNEL=${CHANNEL:-stable}

LDFLAGS="-X github.com/pulak-ranjan/sampmail/internal/core.Version=${VERSION}"
LDFLAGS="${LDFLAGS} -X github.com/pulak-ranjan/sampmail/internal/core.BuildTime=${BUILD_TIME}"
LDFLAGS="${LDFLAGS} -X github.com/pulak-ranjan/sampmail/internal/core.GitCommit=${GIT_COMMIT}"
LDFLAGS="${LDFLAGS} -X github.com/pulak-ranjan/sampmail/internal/core.Channel=${CHANNEL}"
LDFLAGS="${LDFLAGS} -s -w"

# Output directory
mkdir -p dist

# Build frontend first
echo "Building frontend..."
cd web && npm ci && npm run build && cd ..

# Build for different platforms
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
)

for PLATFORM in "${PLATFORMS[@]}"; do
    OS="${PLATFORM%/*}"
    ARCH="${PLATFORM#*/}"
    OUTPUT="dist/sampmail-${VERSION}-${OS}-${ARCH}"
    
    if [ "$OS" = "windows" ]; then
        OUTPUT="${OUTPUT}.exe"
    fi
    
    echo "Building for ${OS}/${ARCH}..."
    GOOS=$OS GOARCH=$ARCH go build -ldflags "${LDFLAGS}" -o "${OUTPUT}" ./cmd/server
done

# Create checksums
echo "Generating checksums..."
cd dist
sha256sum sampmail-* > checksums.txt
cd ..

echo "Build complete! Files in dist/"
ls -la dist/
```

### Run Build

```bash
chmod +x scripts/build.sh
./scripts/build.sh
```

---

## 3. Create Release Package

### Package Script (package.sh)

Create `scripts/package.sh`:

```bash
#!/bin/bash
set -e

VERSION=$(cat VERSION)
DIST_DIR="dist"
RELEASE_DIR="releases"

mkdir -p ${RELEASE_DIR}

# Package each platform
for BINARY in ${DIST_DIR}/sampmail-*; do
    if [[ "$BINARY" == *"checksums"* ]]; then
        continue
    fi
    
    FILENAME=$(basename $BINARY)
    PLATFORM=${FILENAME#sampmail-${VERSION}-}
    PLATFORM=${PLATFORM%.exe}
    
    echo "Packaging ${PLATFORM}..."
    
    # Create temp directory
    TEMP_DIR=$(mktemp -d)
    mkdir -p ${TEMP_DIR}/sampmail
    
    # Copy files
    cp ${BINARY} ${TEMP_DIR}/sampmail/sampmail
    cp -r web/dist ${TEMP_DIR}/sampmail/web 2>/dev/null || true
    cp README.md LICENSE.md CHANGELOG.md ${TEMP_DIR}/sampmail/
    cp .env.example ${TEMP_DIR}/sampmail/
    cp -r scripts ${TEMP_DIR}/sampmail/
    cp -r docs ${TEMP_DIR}/sampmail/
    
    # Create zip
    cd ${TEMP_DIR}
    zip -r sampmail-${VERSION}-${PLATFORM}.zip sampmail/
    mv sampmail-${VERSION}-${PLATFORM}.zip ${OLDPWD}/${RELEASE_DIR}/
    cd ${OLDPWD}
    
    # Cleanup
    rm -rf ${TEMP_DIR}
done

# Copy checksums
cp ${DIST_DIR}/checksums.txt ${RELEASE_DIR}/

# Generate release checksums
cd ${RELEASE_DIR}
sha256sum *.zip > SHA256SUMS.txt
cd ..

echo "Packages created in ${RELEASE_DIR}/"
ls -la ${RELEASE_DIR}/
```

---

## 4. GitHub Release Process

### Step 1: Update Version

```bash
# Edit VERSION file
echo "0.1.12" > VERSION

# Update CHANGELOG.md with release notes
vim CHANGELOG.md
```

### Step 2: Commit and Tag

```bash
# Stage changes
git add -A

# Commit with version
git commit -m "Release v0.1.12

- Feature: Self-update system with dashboard notifications
- Feature: AI-powered template generation
- Feature: Multi-tenant organizations
- Fix: SMTP mutex starvation
- Fix: CSV import memory exhaustion
- Security: Input validation on all imports
- Security: AI prompt injection protection

See CHANGELOG.md for full details."

# Create annotated tag
git tag -a v0.1.12 -m "Release v0.1.12 - Feature release with security fixes"

# Push to GitHub
git push origin main
git push origin v0.1.12
```

### Step 3: Create GitHub Release (CLI)

```bash
# Using GitHub CLI
gh release create v0.1.12 \
  --title "SampMail v0.1.12" \
  --notes-file CHANGELOG.md \
  releases/*.zip \
  releases/SHA256SUMS.txt
```

### Step 3 (Alternative): Create GitHub Release (Web UI)

1. Go to: `https://github.com/pulak-ranjan/sampmail/releases/new`
2. Choose tag: `v0.1.12`
3. Release title: `SampMail v0.1.12`
4. Description: Copy from CHANGELOG.md
5. Upload assets:
   - `sampmail-0.1.12-linux-amd64.zip`
   - `sampmail-0.1.12-linux-arm64.zip`
   - `sampmail-0.1.12-darwin-amd64.zip`
   - `sampmail-0.1.12-darwin-arm64.zip`
   - `SHA256SUMS.txt`
6. Click "Publish release"

---

## 5. Configure Update Server

The updater checks GitHub releases API by default. Update the URL in your `.env`:

```env
# Default: Uses GitHub API
SAMPMAIL_UPDATE_URL=https://api.github.com/repos/pulak-ranjan/sampmail/releases

# Or use custom update server
SAMPMAIL_UPDATE_URL=https://updates.yourdomain.com/api/releases
```

### GitHub Release JSON Structure

The updater expects this structure from GitHub API:

```json
[
  {
    "tag_name": "v0.1.12",
    "name": "SampMail v0.1.12",
    "body": "Release notes here...",
    "prerelease": false,
    "draft": false,
    "published_at": "2025-12-27T00:00:00Z",
    "assets": [
      {
        "name": "sampmail-0.1.12-linux-amd64.zip",
        "size": 15000000,
        "browser_download_url": "https://github.com/.../sampmail-0.1.12-linux-amd64.zip"
      }
    ]
  }
]
```

---

## 6. Complete Release Workflow

### One-Command Release Script (release.sh)

```bash
#!/bin/bash
set -e

# Check for version argument
if [ -z "$1" ]; then
    echo "Usage: ./scripts/release.sh <version>"
    echo "Example: ./scripts/release.sh 0.1.12"
    exit 1
fi

VERSION=$1

echo "=== SampMail Release v${VERSION} ==="

# 1. Update version
echo "${VERSION}" > VERSION
echo "✓ Updated VERSION file"

# 2. Build
echo "Building..."
./scripts/build.sh
echo "✓ Build complete"

# 3. Package
echo "Packaging..."
./scripts/package.sh
echo "✓ Packages created"

# 4. Commit and tag
echo "Committing..."
git add -A
git commit -m "Release v${VERSION}"
git tag -a "v${VERSION}" -m "Release v${VERSION}"
echo "✓ Committed and tagged"

# 5. Push
echo "Pushing to GitHub..."
git push origin main
git push origin "v${VERSION}"
echo "✓ Pushed to GitHub"

# 6. Create release
echo "Creating GitHub release..."
gh release create "v${VERSION}" \
    --title "SampMail v${VERSION}" \
    --notes-file CHANGELOG.md \
    releases/*.zip \
    releases/SHA256SUMS.txt
echo "✓ GitHub release created"

echo ""
echo "=== Release v${VERSION} Complete! ==="
echo "Users will see update notification in dashboard within 24 hours"
echo "Or they can manually check: Settings → Updates → Check for Updates"
```

### Run Release

```bash
chmod +x scripts/release.sh
./scripts/release.sh 0.1.12
```

---

## 7. Verify Release

### Check GitHub Release Page

```
https://github.com/pulak-ranjan/sampmail/releases/tag/v0.1.12
```

### Test Update Detection

```bash
# On a running SampMail instance
curl http://localhost:9000/api/updates/check -X POST

# Response should show:
{
  "success": true,
  "status": {
    "available": true,
    "current_version": "0.1.11",
    "latest_version": "0.1.12",
    "release_info": {...}
  }
}
```

---

## 8. Makefile (Optional)

Create `Makefile` for convenience:

```makefile
VERSION := $(shell cat VERSION)
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -X github.com/pulak-ranjan/sampmail/internal/core.Version=$(VERSION)
LDFLAGS += -X github.com/pulak-ranjan/sampmail/internal/core.BuildTime=$(BUILD_TIME)
LDFLAGS += -X github.com/pulak-ranjan/sampmail/internal/core.GitCommit=$(GIT_COMMIT)
LDFLAGS += -s -w

.PHONY: build dev clean release

build:
	go build -ldflags "$(LDFLAGS)" -o sampmail ./cmd/server

dev:
	go run ./cmd/server

frontend:
	cd web && npm install && npm run build

clean:
	rm -rf sampmail dist/ releases/

release: clean
	./scripts/build.sh
	./scripts/package.sh

version:
	@echo $(VERSION)
```

Usage:

```bash
make build      # Build binary
make dev        # Run in dev mode
make frontend   # Build frontend
make release    # Full release build
make version    # Show version
```

---

## Summary

| Step | Command |
|------|---------|
| Update version | `echo "0.1.12" > VERSION` |
| Build | `./scripts/build.sh` |
| Package | `./scripts/package.sh` |
| Tag | `git tag -a v0.1.12 -m "Release"` |
| Push | `git push origin main --tags` |
| Release | `gh release create v0.1.12 releases/*` |

Users will automatically see updates in their dashboard!
