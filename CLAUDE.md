# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`kubeasy-cli` is a command-line tool built with Go and Cobra that helps developers learn Kubernetes through practical challenges. It manages local Kind clusters, deploys challenges via ArgoCD, and validates solutions using a Kubernetes operator.

## Build, Test, and Development Commands

### Building

```bash
# Build the binary
go build -o kubeasy-cli

# Build with dependencies vendored
go mod vendor
go build -o kubeasy-cli
```

### Linting

```bash
# Run linting (matches CI workflow)
go mod vendor
# Uses super-linter in CI - see .github/workflows/lint.yml
```

### Dependencies

```bash
# Download dependencies
go mod download

# Vendor dependencies (required for linting and private repos)
go mod vendor

# Update dependencies
go mod tidy
```

### Running Locally

```bash
# Run directly
go run main.go [command]

# With debug logging
go run main.go --debug [command]
```

## Architecture

### Command Structure (Cobra-based)

- **Entry point**: `main.go` → `cmd.Execute()`
- **Root command**: `cmd/root.go` - Initializes logging, supports `--debug` flag
- **Commands organized under `cmd/`**:
  - `setup.go` - Creates Kind cluster "kubeasy" and installs ArgoCD + dependencies
  - `login.go` - Stores API key in system keyring (uses `zalando/go-keyring`)
  - `challenge` (parent command in `challenge.go`):
    - `start.go` - Deploys challenge via ArgoCD, creates namespace, tracks progress
    - `submit.go` - Validates solutions by fetching all 6 validation CRD types and submitting results
    - `reset.go` - Deletes resources and resets progress in backend
    - `clean.go` - Removes challenge resources without resetting backend
    - `get.go` - Displays challenge details
  - `common.go` - Shared helper functions for commands

### Core Packages (pkg/)

#### `pkg/api/api.go`

- Communicates with backend API (Next.js + tRPC)
- Authentication via JWT tokens stored in keyring
- Key functions:
  - `createSupabaseClient()` - Retrieves token from keyring
  - `getUserIDFromKeyring()` - Extracts user ID from JWT claims
  - `GetChallenge(slug)` - Fetches challenge metadata
  - `GetChallengeProgress(slug)` - Checks user's progress
  - `StartChallenge(slug)` - Creates progress record
  - `SendSubmit(challengeSlug, validations)` - Submits validation results with new structure
    - Accepts `validations map[string]interface{}` containing all validation types
    - Automatically determines if all validations passed
    - Sends structured payload: `{validated: bool, validations: {...}}`
  - `GetProfile()` - Fetches user profile information

#### `pkg/argocd/`

- `install.go` - ArgoCD installation and health checking
  - `InstallArgoCD(options)` - Installs core components + App-of-Apps pattern
  - `WaitForArgoCDAppsReadyCore(appNames, timeout)` - Waits for apps to be Healthy/Synced
  - `IsArgoCDInstalled()` - Checks if ArgoCD is already present
- `application.go` - Challenge deployment management (creates ArgoCD Applications)
- `const.go` - Constants (namespace, manifest URLs)

#### `pkg/kube/`

- `client.go` - Kubernetes client creation (uses `kind-kubeasy` context)
- `config.go` - Kubeconfig manipulation (namespace switching, context selection)
- `manifest.go` - Manifest fetching and applying (supports dynamic resource creation)

#### `pkg/constants/const.go`

- Global constants:
  - `KubeasyClusterContext = "kind-kubeasy"`
  - `KeyringServiceName = "kubeasy-cli"`
  - `RestAPIUrl` - API endpoint
  - `LogFilePath` - Path for debug logs

#### `pkg/logger/logger.go`

- Custom logging utility with file output support
- Levels: DEBUG, INFO, WARN, ERROR
- Controlled via `--debug` flag on root command

### Key Workflows

#### Challenge Lifecycle

1. **Setup**: `kubeasy setup` → Creates Kind cluster → Installs ArgoCD → Deploys operator/Kyverno
2. **Start**: `kubeasy challenge start <slug>` → Creates namespace → Deploys ArgoCD app → Tracks progress
3. **Work**: User modifies cluster resources manually
4. **Submit**: `kubeasy challenge submit <slug>` → Reads operator CRDs → Validates → Sends results to API
5. **Clean/Reset**: `kubeasy challenge clean/reset <slug>` → Deletes resources ± backend data

#### Authentication Flow

- User runs `kubeasy login` → Enters API key (JWT) → Stored in system keyring
- Token reuse: If valid token exists, prompts to reuse with expiration info
- All API calls retrieve token from keyring and include in Supabase client

#### Validation System

The CLI works with the challenge operator which creates specialized validation CRDs in the challenge namespace:

**Validation Types (all v1alpha1)**:
- `LogValidation` - Validates container logs contain expected strings
- `StatusValidation` - Checks resource status conditions (e.g., Pod Ready, Deployment Available)
- `EventValidation` - Detects forbidden Kubernetes events (e.g., BackOff, Evicted)
- `MetricsValidation` - Validates pod/deployment metrics (restartCount, replicas, etc.)
- `RBACValidation` - Verifies ServiceAccount permissions using SubjectAccessReview
- `ConnectivityValidation` - Tests network connectivity between pods via curl

**Submit Flow**:
1. `submit` command discovers all validation CRDs in the challenge namespace
2. For each validation type, reads status and collects results
3. Builds structured payload: `{logvalidations: [{name, passed, details, rawStatus}], statusvalidations: [...], ...}`
4. Sends to backend API with `validated: bool` (true if all validations passed)
5. Backend stores individual validation results in database for progress tracking

**Legacy Compatibility**:
- Old `StaticValidation` and `DynamicValidation` CRDs have been removed
- Backend still accepts legacy fields for backward compatibility but prioritizes new structure

## Important Implementation Details

### Context Management

- **Always use**: `constants.KubeasyClusterContext` ("kind-kubeasy") when getting Kubernetes clients
- Namespace is set per-challenge in kubeconfig context
- `kube.SetNamespaceForContext()` updates namespace without changing context

### ArgoCD Integration

- App-of-Apps pattern: `cli-setup` repository contains bootstrap manifests
- Main bootstrap app: `kubeasy-cli-setup` (installs Kyverno, operator, ArgoCD itself)
- Challenge apps created in `argocd` namespace, deploy to challenge-specific namespaces

### Error Handling

- Commands use `getChallengeOrExit(slug)` for consistent error handling
- API errors suggest running `kubeasy login` when authentication fails
- Logging via `logger` package writes to file when `--debug` is enabled

### Dependencies

- Uses vendoring for private dependency: `github.com/kubeasy-dev/challenge-operator`
- CI workflows configure GOPRIVATE and GitHub token for access
- Must run `go mod vendor` before linting or building in CI

## Release Process

- Triggered by pushing tags
- Uses GoReleaser for multi-platform builds
- Publishes to:
  - GitHub Releases (binaries + checksums)
  - NPM (via `npm publish`)
  - Cloudflare R2 (AWS S3-compatible storage)
- Go version: 1.24.9 (specified in go.mod and CI)

## Related Repositories

- **challenge-operator** - Kubernetes operator for validation CRDs
- **cli-setup** - ArgoCD manifests for bootstrapping local environment
- **site** - Next.js frontend for browsing challenges
