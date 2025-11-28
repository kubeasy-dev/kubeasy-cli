# CI/CD Pipeline

This document describes the continuous integration and deployment pipeline for `kubeasy-cli`.

## Overview

The CI/CD pipeline is built with GitHub Actions and handles:
- ✅ Code quality checks (linting, testing)
- 📦 Multi-platform binary builds
- 🚀 Automated releases to GitHub and NPM
- 🔄 Automated dependency updates

## Workflows

### 1. Linting (`lint.yml`)

**Triggers:** Push to `main`, Pull Requests

**Purpose:** Ensure code quality and consistency

**Steps:**
1. Run `golangci-lint` with project configuration
2. Check for common Go issues (unused code, complexity, etc.)
3. Validate GitHub Actions workflow files

**Configuration:** `.github/linters/.golangci.yml`

### 2. Manual Release (`release-dispatch.yml`)

**Triggers:** Manual workflow dispatch

**Purpose:** Create new releases with version bump

**Usage:**
1. Navigate to [Actions → Manual Release](https://github.com/kubeasy-dev/kubeasy-cli/actions/workflows/release-dispatch.yml)
2. Click "Run workflow"
3. Select version type:
   - `patch` - Bug fixes (1.0.0 → 1.0.1)
   - `minor` - New features (1.0.0 → 1.1.0)
   - `major` - Breaking changes (1.0.0 → 2.0.0)

**What happens:**
1. **Pre-release validation:**
   - Verify on `main` branch
   - Check working directory is clean
   - Run tests with race detector
   - Run linters
   - Test build
2. **Version bump:**
   - Calculate new version
   - Update `package.json` and `package-lock.json`
   - Create commit: `chore: release vX.Y.Z`
   - Create tag: `vX.Y.Z`
3. **Trigger release:**
   - Push commit and tag to GitHub
   - Automatically triggers the release workflow

### 3. Release Build (`releasing.yml`)

**Triggers:** Git tags matching `v*.*.*` (e.g., `v1.0.0`)

**Purpose:** Build binaries and publish to all platforms

**Steps:**

#### Pre-release Checks
- Run full test suite
- Run linters
- Test build

#### Build Phase (GoReleaser)
- Build binaries for 6 platforms:
  - Linux: amd64, arm64
  - macOS: amd64 (Intel), arm64 (Apple Silicon)
  - Windows: amd64, arm64
- Generate checksums
- Upload binaries to Cloudflare R2
- Create GitHub Release with artifacts

#### NPM Publish Phase
- Wait for binaries to be available on R2
- Run `npm ci` and `npm publish`
- Publish `@kubeasy-dev/kubeasy-cli` package

**Artifacts:**
- GitHub Release with binaries and checksums
- NPM package with binary downloader
- Binaries on `https://download.kubeasy.dev`

### 4. AI Code Review (`claude-code-review.yml`)

**Triggers:** Pull Requests

**Purpose:** AI-powered code review with Claude

**What it does:**
- Analyzes code changes
- Provides suggestions for improvements
- Identifies potential issues

## Release Process

### Automated Release (Recommended)

The easiest way to create a release:

```
1. Go to GitHub Actions → Manual Release
2. Click "Run workflow"
3. Select version type (patch/minor/major)
4. Click "Run workflow"
5. Monitor progress in GitHub Actions
```

### Manual Release (Alternative)

If you need more control:

```bash
# 1. Ensure you're on main and up to date
git checkout main
git pull origin main

# 2. Run pre-release checks
go test -v -race ./...
golangci-lint run --config .github/linters/.golangci.yml
go build .

# 3. Create version and tag
npm version patch  # or minor/major
git push --follow-tags
```

## Configuration Files

### `.goreleaser.yaml`

Defines the release build process:
- Binary build settings (LDFLAGS, platforms, architectures)
- Archive formats (tar.gz for Unix, zip for Windows)
- Cloudflare R2 upload configuration
- Changelog generation rules

Key features:
- Cross-compilation for all platforms
- Version injection via LDFLAGS
- Custom binary names and paths
- Automated changelog from commits

### `package.json`

NPM package configuration:
- Package metadata and version
- Binary download script in `postinstall`
- Release scripts

The NPM package acts as a wrapper that:
1. Detects user's platform (OS + architecture)
2. Downloads the correct binary from R2
3. Installs it globally as `kubeasy`

## Monitoring Releases

### GitHub Actions

Monitor workflow runs:
- https://github.com/kubeasy-dev/kubeasy-cli/actions

Check for:
- ✅ Green checkmarks (success)
- ❌ Red X marks (failure)
- ⏳ Yellow dots (in progress)

### GitHub Releases

View published releases:
- https://github.com/kubeasy-dev/kubeasy-cli/releases

Each release includes:
- Release notes (auto-generated from commits)
- Binary downloads for all platforms
- Checksums file (`checksums.txt`)

### NPM Package

Check NPM publication:
- https://www.npmjs.com/package/@kubeasy-dev/kubeasy-cli

Verify:
- Latest version is published
- Package size is reasonable (~5-10 KB)
- Install works: `npm install -g @kubeasy-dev/kubeasy-cli`

## Troubleshooting

### Release Workflow Fails on Pre-checks

**Problem:** Tests or linters fail

**Solution:**
```bash
# Run locally to identify issue
go test -v ./...
golangci-lint run --config .github/linters/.golangci.yml

# Fix issues, commit, and retry
```

### NPM Publish Fails

**Problem:** Package already exists or authentication fails

**Solutions:**
- Check if version already exists on NPM
- Verify `NPM_TOKEN` secret is valid
- Ensure token has publish permissions

### Binary Download Fails

**Problem:** Users can't download binaries after NPM install

**Solutions:**
- Verify binaries are uploaded to R2: `https://download.kubeasy.dev/kubeasy-cli/vX.Y.Z/`
- Check R2 credentials are valid
- Ensure sufficient wait time in NPM publish job

### GoReleaser Build Fails

**Problem:** Build fails for specific platform

**Solutions:**
- Test locally: `make release-local`
- Check `.goreleaser.yaml` configuration
- Verify LDFLAGS and build tags

## Security

### Secrets Management

Required secrets in GitHub repository settings:
- `GITHUB_TOKEN` - Auto-provided, used for releases
- `NPM_TOKEN` - NPM authentication for publishing
- `R2_ACCESS_KEY_ID` - Cloudflare R2 access key
- `R2_SECRET_ACCESS_KEY` - Cloudflare R2 secret key

### Permissions

Workflows use minimal permissions:
- `contents: read` - Read repository code
- `contents: write` - Create releases and tags
- `id-token: write` - OIDC authentication

### Action Pinning

All GitHub Actions are pinned to specific SHA hashes for security:
```yaml
uses: actions/checkout@1af3b93b6815bc44a9784bd300feb67ff0d1eeb3 # v6.0.0
```

This prevents supply chain attacks from compromised action versions.

## Performance Optimizations

### Caching

Workflows use caching to speed up builds:
- **Go modules cache:** Dependencies are cached between runs
- **Build cache:** Go build artifacts are cached
- **Action cache:** Downloaded actions are cached

Typical speedup: 2-3 minutes per workflow run

### Parallel Jobs

Jobs run in parallel when possible:
- Pre-release checks run before build
- Build and NPM publish can run in parallel (NPM waits for R2)

### Smart Dependencies

- Use `go mod download` explicitly for better caching
- Only download dependencies when `go.sum` changes

## Best Practices

### Before Creating a Release

1. ✅ All tests pass locally
2. ✅ No linter warnings
3. ✅ Changes are merged to `main`
4. ✅ CHANGELOG or commit messages are clear
5. ✅ Version bump type is appropriate

### Semantic Versioning

Follow [semver](https://semver.org/):
- **Patch** (1.0.0 → 1.0.1) - Backward-compatible bug fixes
- **Minor** (1.0.0 → 1.1.0) - Backward-compatible features
- **Major** (1.0.0 → 2.0.0) - Breaking changes

### Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/):
```
feat: add new command
fix: resolve authentication issue
docs: update installation guide
chore: update dependencies
```

These are used to auto-generate release notes.

## Local Testing

### Test Release Build

Test the full release process without publishing:

```bash
# Install GoReleaser
brew install goreleaser

# Run snapshot build
make release-local

# Check artifacts
ls -lh dist/
```

This creates binaries in `dist/` without creating a release.

### Test NPM Package Locally

Test the NPM package installation:

```bash
# Build binaries
make build

# Test NPM package
npm pack
npm install -g kubeasy-dev-kubeasy-cli-*.tgz

# Verify installation
kubeasy version
```

## Related Documentation

- [CONTRIBUTING.md](../CONTRIBUTING.md) - Development workflow
- [README.md](../README.md) - Project overview
- [.github/workflows/README.md](../.github/workflows/README.md) - Workflow details
