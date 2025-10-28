#!/bin/bash
set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Kubeasy CLI Release Script${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Get version type from argument or ask
VERSION_TYPE=${1:-}
if [ -z "$VERSION_TYPE" ]; then
	echo -e "${YELLOW}Select release type:${NC}"
	echo "  1) patch (bug fixes)"
	echo "  2) minor (new features)"
	echo "  3) major (breaking changes)"
	echo ""
	read -r -p "Enter choice (1-3): " choice

	case $choice in
	1) VERSION_TYPE="patch" ;;
	2) VERSION_TYPE="minor" ;;
	3) VERSION_TYPE="major" ;;
	*)
		echo -e "${RED}✗ Invalid choice${NC}"
		exit 1
		;;
	esac
fi

# Validate version type
if [[ ! "$VERSION_TYPE" =~ ^(patch|minor|major)$ ]]; then
	echo -e "${RED}✗ Invalid version type: $VERSION_TYPE${NC}"
	echo -e "${YELLOW}Usage: $0 [patch|minor|major]${NC}"
	exit 1
fi

echo -e "${YELLOW}Running pre-release checks...${NC}"
echo ""

# Verify we're on main branch
BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ "$BRANCH" != "main" ]; then
	echo -e "${RED}✗ You must be on the main branch${NC}"
	echo -e "  Current branch: $BRANCH"
	exit 1
fi
echo -e "${GREEN}✓ On main branch${NC}"

# Verify no uncommitted changes
if [ -n "$(git status --porcelain)" ]; then
	echo -e "${RED}✗ You have uncommitted changes${NC}"
	git status --short
	exit 1
fi
echo -e "${GREEN}✓ Working directory clean${NC}"

# Verify branch is up to date
echo -e "${YELLOW}  Fetching latest from origin...${NC}"
git fetch origin main --quiet
if [ "$(git rev-parse HEAD)" != "$(git rev-parse '@{u}')" ]; then
	echo -e "${RED}✗ Your branch is not up to date with origin/main${NC}"
	echo -e "  Run: git pull origin main"
	exit 1
fi
echo -e "${GREEN}✓ Branch up to date${NC}"

echo ""
echo -e "${YELLOW}Running tests and linters...${NC}"

# Run tests
if ! make test >/dev/null 2>&1; then
	echo -e "${RED}✗ Tests failed${NC}"
	echo -e "  Run: make test"
	exit 1
fi
echo -e "${GREEN}✓ Tests passed${NC}"

# Run linters
if ! make lint >/dev/null 2>&1; then
	echo -e "${RED}✗ Linting failed${NC}"
	echo -e "  Run: make lint"
	exit 1
fi
echo -e "${GREEN}✓ Linting passed${NC}"

# Test build
echo -e "${YELLOW}  Building...${NC}"
if ! make build >/dev/null 2>&1; then
	echo -e "${RED}✗ Build failed${NC}"
	echo -e "  Run: make build"
	exit 1
fi
echo -e "${GREEN}✓ Build successful${NC}"

echo ""
echo -e "${BLUE}========================================${NC}"

# Get version information
CURRENT_VERSION=$(node -p "require('./package.json').version")

# Calculate new version manually instead of using npm --dry-run (which has side effects)
IFS='.' read -r -a version_parts <<<"$CURRENT_VERSION"
major="${version_parts[0]}"
minor="${version_parts[1]}"
patch="${version_parts[2]}"

case "$VERSION_TYPE" in
"patch")
	patch=$((patch + 1))
	;;
"minor")
	minor=$((minor + 1))
	patch=0
	;;
"major")
	major=$((major + 1))
	minor=0
	patch=0
	;;
esac

NEW_VERSION="v${major}.${minor}.${patch}"

echo -e "${YELLOW}Current version:${NC} $CURRENT_VERSION"
echo -e "${GREEN}New version:    ${NC} $NEW_VERSION"
echo ""
echo -e "${BLUE}========================================${NC}"
echo ""

# Confirm
echo -e "${YELLOW}This will:${NC}"
echo "  1. Update package.json to $NEW_VERSION"
echo "  2. Create a git commit with the version bump"
echo "  3. Create a git tag $NEW_VERSION"
echo "  4. Push the commit and tag to GitHub"
echo "  5. Trigger the CI/CD pipeline for release"
echo ""
read -r -p "$(echo -e "${YELLOW}Continue? [y/N]:${NC} ")" CONFIRM

if [ "$CONFIRM" != "y" ] && [ "$CONFIRM" != "Y" ]; then
	echo -e "${RED}✗ Release cancelled${NC}"
	exit 1
fi

echo ""
echo -e "${GREEN}✓ Creating release...${NC}"

# Update version in package.json without creating git commit/tag
# (we'll do that manually to have more control)
npm version "$VERSION_TYPE" --no-git-tag-version >/dev/null 2>&1

# Create git commit and tag manually
git add package.json package-lock.json
git commit -m "chore: release $NEW_VERSION"
git tag -a "$NEW_VERSION" -m "Release $NEW_VERSION"

echo ""
echo -e "${GREEN}✓ Version bumped to $NEW_VERSION${NC}"
echo ""
echo -e "${YELLOW}Pushing to GitHub...${NC}"

# Push with tags
if ! git push --follow-tags; then
	echo ""
	echo -e "${RED}✗ Failed to push to GitHub${NC}"
	echo -e "${YELLOW}You can try again manually:${NC}"
	echo -e "  git push --follow-tags"
	exit 1
fi

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  ✓ Release initiated successfully!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${BLUE}Next steps:${NC}"
echo -e "  1. Monitor the release: ${BLUE}https://github.com/kubeasy-dev/kubeasy-cli/actions${NC}"
echo -e "  2. Check the release page: ${BLUE}https://github.com/kubeasy-dev/kubeasy-cli/releases/tag/$NEW_VERSION${NC}"
echo -e "  3. Verify NPM publish: ${BLUE}https://www.npmjs.com/package/@kubeasy-dev/kubeasy-cli${NC}"
echo ""
