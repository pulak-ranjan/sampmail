#!/bin/bash
set -e

# SampMail Release Script
# Creates a complete release and publishes to GitHub

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check for version argument
if [ -z "$1" ]; then
    echo -e "${YELLOW}Usage: ./scripts/release.sh <version>${NC}"
    echo "Example: ./scripts/release.sh 0.1.12"
    echo ""
    echo "Current version: $(cat VERSION)"
    exit 1
fi

VERSION=$1

# Validate version format
if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo -e "${RED}Error: Version must be in format X.Y.Z (e.g., 0.1.12)${NC}"
    exit 1
fi

echo "========================================"
echo "  SampMail Release v${VERSION}"
echo "========================================"
echo ""

# Check for uncommitted changes
if [ -n "$(git status --porcelain)" ]; then
    echo -e "${YELLOW}Warning: You have uncommitted changes${NC}"
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Check if tag already exists
if git tag -l | grep -q "^v${VERSION}$"; then
    echo -e "${RED}Error: Tag v${VERSION} already exists${NC}"
    exit 1
fi

# 1. Update VERSION file
echo "1. Updating VERSION file..."
echo "${VERSION}" > VERSION
echo -e "${GREEN}✓ VERSION updated to ${VERSION}${NC}"

# 2. Build
echo ""
echo "2. Building binaries..."
./scripts/build.sh
echo -e "${GREEN}✓ Build complete${NC}"

# 3. Create packages
echo ""
echo "3. Creating release packages..."

RELEASE_DIR="releases"
mkdir -p ${RELEASE_DIR}

# Package each binary
for BINARY in dist/sampmail-*; do
    if [[ "$BINARY" == *"checksums"* ]]; then
        continue
    fi
    
    FILENAME=$(basename $BINARY)
    PLATFORM=${FILENAME#sampmail-${VERSION}-}
    
    echo "  Packaging ${PLATFORM}..."
    
    # Create temp directory
    TEMP_DIR=$(mktemp -d)
    mkdir -p ${TEMP_DIR}/sampmail
    
    # Copy binary
    cp ${BINARY} ${TEMP_DIR}/sampmail/sampmail
    chmod +x ${TEMP_DIR}/sampmail/sampmail
    
    # Copy web assets if they exist
    if [ -d "web/dist" ]; then
        cp -r web/dist ${TEMP_DIR}/sampmail/web
    fi
    
    # Copy documentation
    cp README.md ${TEMP_DIR}/sampmail/ 2>/dev/null || true
    cp LICENSE.md ${TEMP_DIR}/sampmail/ 2>/dev/null || true
    cp CHANGELOG.md ${TEMP_DIR}/sampmail/ 2>/dev/null || true
    cp .env.example ${TEMP_DIR}/sampmail/ 2>/dev/null || true
    
    # Copy scripts
    if [ -d "scripts" ]; then
        mkdir -p ${TEMP_DIR}/sampmail/scripts
        cp scripts/install*.sh ${TEMP_DIR}/sampmail/scripts/ 2>/dev/null || true
    fi
    
    # Create tar.gz
    TARNAME="sampmail-${PLATFORM}.tar.gz"
    cd ${TEMP_DIR}
    tar -czf sampmail.tar.gz sampmail/
    mv sampmail.tar.gz ${OLDPWD}/${RELEASE_DIR}/${TARNAME}
    cd ${OLDPWD}
    
    # Cleanup
    rm -rf ${TEMP_DIR}
    
    echo -e "${GREEN}  ✓ Created ${TARNAME}${NC}"
done

# Generate checksums for releases
cd ${RELEASE_DIR}
sha256sum *.tar.gz > SHA256SUMS.txt
cd ..

echo -e "${GREEN}✓ Packages created${NC}"

# 4. Git commit and tag
echo ""
echo "4. Committing and tagging..."
git add -A
git commit -m "Release v${VERSION}

See CHANGELOG.md for details."
git tag -a "v${VERSION}" -m "Release v${VERSION}"
echo -e "${GREEN}✓ Committed and tagged${NC}"

# 5. Push to GitHub
echo ""
echo "5. Pushing to GitHub..."
read -p "Push to GitHub? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    git push origin main
    git push origin "v${VERSION}"
    echo -e "${GREEN}✓ Pushed to GitHub${NC}"
    
    # 6. Create GitHub release
    echo ""
    echo "6. Creating GitHub release..."
    
    # Check if gh CLI is available
    if command -v gh &> /dev/null; then
        read -p "Create GitHub release with gh CLI? (y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            gh release create "v${VERSION}" \
                --title "SampMail v${VERSION}" \
                --notes-file CHANGELOG.md \
                ${RELEASE_DIR}/*.tar.gz \
                ${RELEASE_DIR}/SHA256SUMS.txt
            echo -e "${GREEN}✓ GitHub release created${NC}"
        fi
    else
        echo -e "${YELLOW}GitHub CLI (gh) not found. Create release manually:${NC}"
        echo "  1. Go to: https://github.com/pulak-ranjan/sampmail/releases/new"
        echo "  2. Choose tag: v${VERSION}"
        echo "  3. Upload files from: ${RELEASE_DIR}/"
    fi
else
    echo -e "${YELLOW}Skipped pushing. Run manually:${NC}"
    echo "  git push origin main"
    echo "  git push origin v${VERSION}"
fi

echo ""
echo "========================================"
echo -e "${GREEN}  Release v${VERSION} Complete!${NC}"
echo "========================================"
echo ""
echo "Release files: ${RELEASE_DIR}/"
ls -la ${RELEASE_DIR}/
echo ""
echo "Users will see update notification within 24 hours,"
echo "or can manually check: Settings → Updates → Check for Updates"
