# Technology Stack

**Analysis Date:** 2026-03-09

## Languages

**Primary:**
- Go 1.25.4 (go.mod) / 1.26.1 (CI) - All application code in `cmd/`, `internal/`

**Secondary:**
- JavaScript (Node.js) - NPM distribution wrapper (`install.js`, `uninstall.js`, `package.json`)
- YAML - Configuration, manifests, task automation (`Taskfile.yml`, `.goreleaser.yaml`)

## Runtime

**Environment:**
- Go runtime (no external server runtime required - CLI binary)
- Node.js 24.14.0 (CI only, for NPM publishing)

**Package Manager:**
- Go modules (`go.mod` / `go.sum`)
- npm for JavaScript distribution wrapper
- Lockfile: `go.sum` present; `package-lock.json` present

## Frameworks

**Core:**
- `github.com/spf13/cobra v1.10.2` - CLI command framework, entry point via `cmd/root.go`
- `sigs.k8s.io/kind v0.31.0` - Kubernetes-in-Docker cluster management
- `k8s.io/client-go v0.35.0` - Kubernetes API client
- `sigs.k8s.io/controller-runtime v0.23.3` - Controller utilities; also provides `setup-envtest` for integration testing

**UI/Display:**
- `github.com/pterm/pterm v0.12.83` - Terminal UI (spinners, tables, styled output) used in `internal/ui/`

**Testing:**
- `github.com/stretchr/testify v1.11.1` - Assertions and test suites
- `sigs.k8s.io/controller-runtime/tools/setup-envtest` - Kubernetes API server for integration tests

**Build/Dev:**
- [Taskfile](https://taskfile.dev) v3 - Task automation (`Taskfile.yml`)
- GoReleaser v2 - Cross-platform binary release (`.goreleaser.yaml`)
- `oapi-codegen v2.5.1` - Generates Go HTTP client from OpenAPI spec (`openapi.json` → `internal/apigen/client.gen.go`)
- `golangci-lint` (latest) - Linting with `gofmt`, `goimports`, `errcheck`, `gosec`, `staticcheck` and others (`.github/linters/.golangci.yml`)

## Key Dependencies

**Critical:**
- `oras.land/oras-go/v2 v2.6.0` - OCI artifact pull for challenge distribution from `ghcr.io`
- `github.com/zalando/go-keyring v0.2.6` - System keyring integration for API key storage (used in `internal/keystore/`)
- `github.com/golang-jwt/jwt/v5 v5.3.1` - JWT parsing for API key validation
- `github.com/hypersequent/zen v0.0.0-20250923135653-056103bb12ce` - Functional utilities (used in validation executor)
- `github.com/oapi-codegen/runtime v1.2.0` - Runtime support for generated API client

**Infrastructure:**
- `k8s.io/api v0.35.0` - Kubernetes API types
- `k8s.io/apimachinery v0.35.0` - Kubernetes API machinery
- `k8s.io/klog/v2 v2.130.1` - Kubernetes logging (suppressed/redirected in CLI context)
- `go.yaml.in/yaml/v3 v3.0.4` - YAML parsing for `challenge.yaml` validation configs
- `golang.org/x/term v0.40.0` - Terminal detection for interactive prompts
- `golang.org/x/sys v0.42.0` - OS-level syscalls

## Configuration

**Environment:**
- `KUBEASY_API_KEY` - API token override (highest priority, for CI/CD)
- No `.env` files; credentials stored in system keyring or `~/.config/kubeasy-cli/credentials` (XDG)
- Cluster context always `kind-kubeasy` (hardcoded in `internal/constants/const.go`)

**Build:**
- `Taskfile.yml` - Developer tasks (build, test, lint, release)
- `.goreleaser.yaml` - Release configuration
- `go.mod` - Module dependencies
- `internal/constants/const.go` - Runtime constants (versions, URLs, cluster name)
- `internal/deployer/const.go` - Deployer-specific versions (Kyverno, local-path-provisioner)
- Build-time variable injection via `-ldflags`:
  - `constants.Version` - Binary version tag
  - `constants.LogFilePath` - Default log file path (`/tmp/kubeasy-cli.log`)
  - `constants.WebsiteURL` - Backend API base URL (`https://kubeasy.dev`)
  - `constants.ExercicesRepoBranch` - Challenges repo branch (`main`)

## Platform Requirements

**Development:**
- Go 1.25.4+ (go.mod minimum), Go 1.26.1 used in CI
- Taskfile CLI (`task`)
- Docker (for Kind cluster creation and local testing)
- `setup-envtest` (auto-installed via `task install:envtest`) for integration tests
- `golangci-lint` (auto-installed via `task install:lint`)

**Production:**
- Binary distributed via:
  - GitHub Releases (tar.gz for linux/darwin/windows × amd64/arm64)
  - NPM package `@kubeasy-dev/kubeasy-cli` (installs binary via postinstall script)
  - Homebrew Cask (`kubeasy-dev/homebrew-tap`)
  - Scoop bucket (`kubeasy-dev/scoop-bucket`)
  - Cloudflare R2 CDN (`https://download.kubeasy.dev`)
- Requires Docker at runtime (Kind cluster creation)
- Target platforms: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64

---

*Stack analysis: 2026-03-09*
