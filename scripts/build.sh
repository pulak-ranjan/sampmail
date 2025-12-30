#!/bin/bash
set -e

# SampMail Build Script
# Builds binaries for multiple platforms with version info embedded

VERSION=$(cat VERSION)
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
CHANNEL=${CHANNEL:-stable}

echo "=== SampMail Build ==="
echo "Version: ${VERSION}"
echo "Build Time: ${BUILD_TIME}"
echo "Git Commit: ${GIT_COMMIT}"
echo "Channel: ${CHANNEL}"
echo ""

# Build ldflags
LDFLAGS="-X github.com/pulak-ranjan/sampmail/internal/core.Version=${VERSION}"
LDFLAGS="${LDFLAGS} -X github.com/pulak-ranjan/sampmail/internal/core.BuildTime=${BUILD_TIME}"
LDFLAGS="${LDFLAGS} -X github.com/pulak-ranjan/sampmail/internal/core.GitCommit=${GIT_COMMIT}"
LDFLAGS="${LDFLAGS} -X github.com/pulak-ranjan/sampmail/internal/core.Channel=${CHANNEL}"
LDFLAGS="${LDFLAGS} -s -w"  # Strip debug info for smaller binary

# Output directory
mkdir -p dist

# Build frontend
if [ -d "web" ]; then
    echo "Building frontend..."
    cd web
    if [ -f "package-lock.json" ]; then
        npm ci --silent
    else
        npm install --silent
    fi
    npm run build --silent
    cd ..
    echo "✓ Frontend built"
fi

# Platforms to build for
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
)

# Optional: uncomment for macOS builds
# PLATFORMS+=(
#     "darwin/amd64"
#     "darwin/arm64"
# )

# Build for each platform
for PLATFORM in "${PLATFORMS[@]}"; do
    OS="${PLATFORM%/*}"
    ARCH="${PLATFORM#*/}"
    OUTPUT="dist/sampmail-${VERSION}-${OS}-${ARCH}"
    
    if [ "$OS" = "windows" ]; then
        OUTPUT="${OUTPUT}.exe"
    fi
    
    echo "Building for ${OS}/${ARCH}..."
    CGO_ENABLED=0 GOOS=$OS GOARCH=$ARCH go build -ldflags "${LDFLAGS}" -o "${OUTPUT}" ./cmd/server
    
    if [ $? -eq 0 ]; then
        SIZE=$(du -h "${OUTPUT}" | cut -f1)
        echo "✓ Built ${OUTPUT} (${SIZE})"
    else
        echo "✗ Failed to build for ${OS}/${ARCH}"
        exit 1
    fi
done

# Generate checksums
echo ""
echo "Generating checksums..."
cd dist
sha256sum sampmail-* > checksums.txt
cd ..

echo ""
echo "=== Build Complete ==="
ls -la dist/
