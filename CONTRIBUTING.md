# Contributing to Kubeasy CLI

Thank you for considering contributing to Kubeasy CLI! This document provides guidelines and instructions for development.

## Prerequisites

Before you start, ensure you have the following tools installed:

- **Go 1.25.3+** - [Download](https://go.dev/dl/)
- **Node.js 24.10.0+** - For npm scripts and release management
- **Make** - Build automation tool (usually pre-installed on macOS/Linux)
- **golangci-lint** - Go linting tool
- **Docker** - Required for running Kind clusters (local Kubernetes)
- **kubectl** - Kubernetes command-line tool
- **Git** - Version control

### Installing Development Tools

Run the following command to install required development tools:

```bash
make install-tools
```

This will install:

- golangci-lint (Go linter)
- goreleaser (Release automation)

## Getting Started

1. **Clone the repository**

```bash
Git clone https://github.com/kubeasy-dev/kubeasy-cli.git
cd kubeasy-cli
```

2. **Install dependencies**

```bash
make deps
npm ci
```

3. **Build the project**

```bash
make build
```

4. **Run the CLI**

```bash
./bin/kubeasy --help
```

## Development Workflow

### Available Make Commands

Run `make help` to see all available commands:

```bash
make help
```

Key commands:

- `make build` - Build the binary for your current platform
- `make test` - Run tests with coverage
- `make lint` - Run linters
- `make fmt` - Format Go code
- `make clean` - Clean build artifacts
- `make dev` - Build and run in development mode

### Code Style

- Follow standard Go conventions
- Run `make fmt` before committing
- Ensure all linters pass with `make lint`

### Pre-commit Hooks

The project uses Husky for Git hooks. Before committing:

1. Code is automatically formatted with `gofmt`
2. `golangci-lint` runs to check for issues

If the pre-commit hook fails, fix the issues before committing.

### Running Tests

```bash
# Run all tests
make test

# Generate HTML coverage report
make test-coverage
open coverage.html
```

### Working with Vendored Dependencies

This project uses Go modules with vendoring for private dependencies:

```bash
# Update dependencies
make deps

# Regenerate vendor directory (only if needed)
make vendor
```

**Note**: The `vendor/` directory is ignored by Git to keep the repository size small.

## Release Process

### Creating a Release

**Prerequisites**:

- You must be on the `main` branch
- Your branch must be up to date with `origin/main`
- All tests and linters must pass
- No uncommitted changes

**Steps**:

1. **Use the release script**

```bash
./scripts/release.sh
```

Or specify the version type directly:

```bash
./scripts/release.sh patch   # 1.4.0 → 1.4.1
./scripts/release.sh minor   # 1.4.0 → 1.5.0
./scripts/release.sh major   # 1.4.0 → 2.0.0
```

The script will:

- Run all prerelease checks (tests, lint, build)
- Bump the version in `package.json`
- Create a Git commit and tag
- Push to GitHub
- Trigger the CI/CD pipeline

2. **Monitor the release**

Watch the GitHub Actions workflow:

- https://github.com/kubeasy-dev/kubeasy-cli/actions

3. **Verify the release**

```bash
./scripts/check-release.sh
```

This checks:

- GitHub Release exists
- npm package is published
- Cloudflare R2 binaries are available
- Checksums file is present

### Manual Release (Alternative)

If you prefer the manual approach:

```bash
# Run prerelease checks
make release-check

# Create version and tag
npm version patch   # or minor/major

# Push to GitHub
Git push --follow-tags
```

### Testing Release Locally

Before creating an actual release, test the GoReleaser configuration:

```bash
make release-local
```

This creates a snapshot build without publishing.

## CI/CD Workflows

### Lint Workflow

Runs on every push and PR to `main`:

- Go linting with `golangci-lint`
- Additional checks with `super-linter` (GitHub Actions, YAML, etc.)

### Release Workflow

Triggered when a version tag is pushed:

1. **Prerelease checks** - Runs tests, lint, and build
2. **Build** - GoReleaser builds binaries for all platforms
3. **npm Publish** - Publishes to npm (runs in parallel with build)

## Project Structure

```
kubeasy-cli/
├── cmd/                 # Cobra commands
│   ├── root.go         # Root command
│   ├── setup.go        # Cluster setup
│   ├── login.go        # Authentication
│   └── challenge/      # Challenge commands
├── pkg/                # Core packages
│   ├── api/            # Backend API client
│   ├── argocd/         # ArgoCD integration
│   ├── kube/           # Kubernetes utilities
│   ├── constants/      # Constants
│   └── logger/         # Logging utilities
├── scripts/            # Build and release scripts
├── .github/            # GitHub Actions workflows
├── Makefile            # Build automation
└── main.go             # Entry point
```

## Commit Convention

We follow [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` - New feature
- `fix:` - bugfix
- `docs:` - Documentation changes
- `style:` - Code style changes (formatting, etc.)
- `refactor:` - Code refactoring
- `perf:` - Performance improvements
- `test:` - Adding or updating tests
- `chore:` - Maintenance tasks

Examples:

```
feat: add support for multi-cluster management
fix: resolve issue with ArgoCD app sync
docs: update README with new installation steps
chore: bump dependencies to latest versions
```

## Getting Help

- **Issues**: [GitHub Issues](https://github.com/kubeasy-dev/kubeasy-cli/issues)
- **Discussions**: [GitHub Discussions](https://github.com/kubeasy-dev/kubeasy-cli/discussions)
- **Site**: [kubeasy.dev](https://kubeasy.dev)

## License

By contributing to Kubeasy CLI, you agree that your contributions will be licensed under the MIT License.
