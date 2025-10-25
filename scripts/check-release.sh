#!/bin/bash

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Get version from package.json or argument
VERSION=${1:-$(node -p "require('./package.json').version")}

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Release Verification for v${VERSION}${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Check GitHub Release
echo -e "${YELLOW}1. Checking GitHub Release...${NC}"
if gh release view "v$VERSION" >/dev/null 2>&1; then
  echo -e "${GREEN}   ✓ GitHub Release exists${NC}"
  RELEASE_URL="https://github.com/kubeasy-dev/kubeasy-cli/releases/tag/v$VERSION"
  echo -e "${BLUE}   → $RELEASE_URL${NC}"
else
  echo -e "${RED}   ✗ GitHub Release not found${NC}"
fi

echo ""

# Check NPM Package
echo -e "${YELLOW}2. Checking NPM Package...${NC}"
if npm view @kubeasy-dev/kubeasy-cli@$VERSION >/dev/null 2>&1; then
  echo -e "${GREEN}   ✓ NPM package published${NC}"
  NPM_URL="https://www.npmjs.com/package/@kubeasy-dev/kubeasy-cli/v/$VERSION"
  echo -e "${BLUE}   → $NPM_URL${NC}"
else
  echo -e "${RED}   ✗ NPM package not found${NC}"
fi

echo ""

# Check Cloudflare R2 binaries
echo -e "${YELLOW}3. Checking Cloudflare R2 binaries...${NC}"

PLATFORMS=("linux_amd64" "linux_arm64" "darwin_amd64" "darwin_arm64" "windows_amd64" "windows_arm64")
R2_FAILURES=0

for PLATFORM in "${PLATFORMS[@]}"; do
  URL="https://download.kubeasy.dev/kubeasy-cli/v${VERSION}/kubeasy-cli_v${VERSION}_${PLATFORM}.tar.gz"

  if curl -s -I "$URL" 2>&1 | grep -q "200 OK\|302 Found"; then
    echo -e "${GREEN}   ✓ $PLATFORM${NC}"
  else
    echo -e "${RED}   ✗ $PLATFORM${NC}"
    R2_FAILURES=$((R2_FAILURES + 1))
  fi
done

if [ $R2_FAILURES -eq 0 ]; then
  echo -e "${GREEN}   All binaries available${NC}"
else
  echo -e "${RED}   $R2_FAILURES binaries missing${NC}"
fi

echo ""

# Check checksums
echo -e "${YELLOW}4. Checking checksums...${NC}"
CHECKSUMS_URL="https://download.kubeasy.dev/kubeasy-cli/v${VERSION}/checksums.txt"
if curl -s -I "$CHECKSUMS_URL" 2>&1 | grep -q "200 OK\|302 Found"; then
  echo -e "${GREEN}   ✓ Checksums file available${NC}"
  echo -e "${BLUE}   → $CHECKSUMS_URL${NC}"
else
  echo -e "${RED}   ✗ Checksums file not found${NC}"
fi

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${GREEN}  Release verification complete${NC}"
echo -e "${BLUE}========================================${NC}"
