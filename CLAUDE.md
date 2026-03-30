# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`kubeasy-cli` is a command-line tool built with Go and Cobra that helps developers learn Kubernetes through practical challenges. It manages local Kind clusters, deploys challenges via OCI artifacts, and validates solutions using a **CLI-based validation system** (as of v2.0.0).

## Commit & PR Conventions

Commits and PR titles **must** follow [Conventional Commits](https://www.conventionalcommits.org/) with an optional scope:

```
<type>(<scope>): <description>
```

Allowed types: `feat`, `fix`, `docs`, `refactor`, `perf`, `style`, `test`, `ci`, `chore`.

Examples:
- `feat: add cluster status command`
- `fix(deploy): handle missing namespace`
- `chore(deps): bump kyverno to v1.13`

This convention is used by GoReleaser to generate the changelog.

## Documentation

Comprehensive documentation is available in the `docs/` folder:

- **[docs/VALIDATION_EXAMPLES.md](docs/VALIDATION_EXAMPLES.md)** - Complete validation examples for all 5 validation types
- **[CICD.md](docs/CICD.md)** - Continuous Integration and Deployment processes

**Always refer to these documents** when working with the validation system, migrating challenges, or creating new validations.

## Build, Test, and Development Commands

This project uses [Taskfile](https://taskfile.dev) for task automation. Run `task --list` to see all available commands.

### Building

```bash
# Build the binary
task build

# Build for all platforms
task build:all
```

### Testing

```bash
# Run all tests (unit + integration)
task test

# Run unit tests only
task test:unit

# Run integration tests only
task test:integration

# Generate coverage report
task test:coverage
```

### Linting

```bash
# Run golangci-lint
task lint

# Run with auto-fix
task lint:fix

# Format code
task fmt
```

### Dependencies

```bash
# Download and tidy dependencies
task deps

# Generate vendor directory
task vendor
```

### Tool Installation

Tools are installed automatically when needed via status checks, but you can install them manually:

```bash
# Install all development tools
task install:tools

# Install specific tools
task install:lint      # golangci-lint
task install:envtest   # setup-envtest for integration tests
```

### Running Locally

```bash
# Build and run
task dev

# Or run directly
go run main.go [command]

# With debug logging
go run main.go --debug [command]
```

## Architecture

### Command Structure (Cobra-based)

- **Entry point**: `main.go` → `cmd.Execute()`
- **Root command**: `cmd/root.go` - Initializes logging, supports `--debug` flag
- **Commands organized under `cmd/`**:
  - `setup.go` - Creates Kind cluster "kubeasy" and installs infrastructure (Kyverno + local-path-provisioner)
  - `login.go` - Stores API key in system keyring (uses `zalando/go-keyring`)
  - `challenge` (parent command in `challenge.go`):
    - `start.go` - Deploys challenge via OCI artifact, creates namespace, tracks progress
    - `submit.go` - Validates solutions by loading validation specs and submitting results
    - `reset.go` - Deletes resources and resets progress in backend
    - `clean.go` - Removes challenge resources without resetting backend
    - `get.go` - Displays challenge details
  - `common.go` - Shared helper functions for commands

### Core Packages (internal/)

#### `internal/api/api.go`

- Communicates with backend API (Next.js + tRPC)
- Authentication via JWT tokens stored in keyring
- Key functions:
  - `createSupabaseClient()` - Retrieves token from keyring
  - `getUserIDFromKeyring()` - Extracts user ID from JWT claims
  - `GetChallenge(slug)` - Fetches challenge metadata
  - `GetChallengeProgress(slug)` - Checks user's progress
  - `StartChallenge(slug)` - Creates progress record
  - `SendSubmit(challengeSlug, results)` - Submits validation results
    - Accepts `[]ObjectiveResult` with key, passed flag, and message
    - Sends structured payload: `{results: [{objectiveKey, passed, message}, ...]}`
  - `GetProfile()` - Fetches user profile information

#### `internal/deployer/`

Handles direct deployment of infrastructure and challenges without ArgoCD.

- `infrastructure.go` - Installs Kyverno and local-path-provisioner directly via HTTP manifests
  - `SetupInfrastructure()` - Downloads and applies install manifests, waits for readiness
  - `IsInfrastructureReady()` - Checks if all infrastructure deployments are running
  - `IsInfrastructureReadyWithClient(ctx, clientset)` - Testable version with injected client
- `challenge.go` - Deploys challenges via OCI artifacts from ghcr.io
  - `DeployChallenge(ctx, clientset, dynamicClient, slug)` - Pulls OCI artifact, applies manifests, waits for ready
- `cleanup.go` - Cleans up challenge resources
  - `CleanupChallenge(ctx, clientset, slug)` - Deletes namespace and restores kubectl context

#### `internal/validation/`

**New in v1.4.0** - CLI-based validation system

- `loader.go` - Loads validation configs from challenge.yaml
  - `LoadForChallenge(slug)` - Tries local file first, falls back to GitHub
  - `loadFromURL(url)` - Private, validates URLs against trusted base
  - `Parse(data)` - Parses YAML and validates specs
  - Security: URL validation prevents injection attacks

- `executor.go` - Thin router; dispatches to type-specific executor sub-packages
  - `NewExecutor(clientset, dynamicClient, restConfig, namespace)` - Creates executor
  - `Execute(ctx, validation)` - Routes to `executors/<type>/executor.go`
  - `ExecuteAll(ctx, validations)` - Runs all validations in parallel
  - `ExecuteSequential(ctx, validations, failFast)` - Runs validations sequentially

- `types.go` - Re-exports all types and constants from `vtypes/` (type aliases for backward compat)

- `vtypes/types.go` - Leaf package with all spec type definitions (no internal imports)
  - All spec types: `StatusSpec`, `ConditionSpec`, `LogSpec`, `EventSpec`, `ConnectivitySpec`, `RbacSpec`, `SpecSpec`, `TriggeredSpec`, etc.
  - `Result` - Validation result with key, passed flag, and message
  - `RegisteredTypes` - Drives Zod schema generation

- `shared/` - Shared helpers used by multiple executor sub-packages
  - `deps.go` - `Deps` struct (injected clients, namespace, probeMu)
  - `gvr.go` - `GetGVRForKind` (kind → GroupVersionResource mapping)
  - `pods.go` - `GetTargetPods`, `GetPodsForResource`
  - `compare.go` - `CompareValues`, `CompareTypedValues`, `GetNestedInt64`

- `executors/` - One sub-package per validation type, each with `Execute()` and tests
  - `status/`, `condition/`, `log/`, `event/`, `rbac/`, `spec/`, `connectivity/`, `triggered/`

#### `internal/kube/`

- `client.go` - Kubernetes client creation (uses `kind-kubeasy` context)
- `config.go` - Kubeconfig manipulation (namespace switching, context selection)
- `manifest.go` - Manifest fetching and applying (supports dynamic resource creation)

#### `internal/constants/const.go`

- Global constants:
  - `KubeasyClusterContext = "kind-kubeasy"`
  - `KeyringServiceName = "kubeasy-cli"`
  - `RestAPIUrl` - API endpoint
  - `LogFilePath` - Path for debug logs
  - `ChallengesOCIRegistry` - OCI registry for challenge artifacts (ghcr.io)
  - `KyvernoVersion` - Kyverno release version (Renovate-managed)
  - `LocalPathProvisionerVersion` - local-path-provisioner version (Renovate-managed)

#### `internal/logger/logger.go`

- Custom logging utility with file output support
- Levels: DEBUG, INFO, WARN, ERROR
- Controlled via `--debug` flag on root command

### Key Workflows

#### Challenge Lifecycle

1. **Setup**: `kubeasy setup` → Creates Kind cluster → Installs Kyverno + local-path-provisioner
2. **Start**: `kubeasy challenge start <slug>` → Creates namespace → Pulls OCI artifact → Applies manifests → Tracks progress
3. **Work**: User modifies cluster resources manually
4. **Submit**: `kubeasy challenge submit <slug>` → Loads validations from challenge.yaml → Executes checks → Sends results to API
5. **Clean/Reset**: `kubeasy challenge clean/reset <slug>` → Deletes namespace ± backend data

#### Authentication Flow

- User runs `kubeasy login` → Enters API key (JWT) → Stored in system keyring
- Token reuse: If valid token exists, prompts to reuse with expiration info
- All API calls retrieve token from keyring and include in Supabase client

#### Validation System (CLI-Based, v1.4.0+)

The CLI now uses a **self-contained validation executor** that loads validation definitions from `challenge.yaml` and executes checks directly against the cluster. No operator or CRDs required.

**For complete details, see [docs/VALIDATION_EXAMPLES.md](docs/VALIDATION_EXAMPLES.md)**

**Supported Validation Types** (5 types):
1. **condition** - Shorthand for checking Kubernetes conditions (e.g., Pod Ready, Deployment Available)
2. **status** - Validates arbitrary status fields with operators (replicas, restartCount, array access)
3. **log** - Searches container logs for expected strings
4. **event** - Detects forbidden Kubernetes events (OOMKilled, Evicted, BackOff)
5. **connectivity** - Tests HTTP connectivity between pods

**Key Components**:
- **internal/validation/loader.go** - Loads validations from challenge.yaml (local or GitHub)
- **internal/validation/executor.go** - Executes validations directly against cluster
- **internal/validation/types.go** - Type definitions for all validation specs

**Submit Flow**:
1. `submit` command loads validations from `challenge.yaml`
2. Executor runs each validation against the cluster
3. Builds results: `{results: [{objectiveKey, passed, message}, ...]}`
4. Sends to backend API
5. Backend validates all expected objectives are present and stores results

**Migration Notes**:
- Old operator-based system (≤v1.3.0) used CRDs
- New CLI-based system (≥v1.4.0) loads from challenge.yaml
- See [docs/MIGRATION.md](docs/MIGRATION.md) for complete migration guide
- RBAC validation type re-introduced (v2.x) using `SubjectAccessReview` with scoped checks. The original removal was due to an operator-based implementation that required privileged CRD access. The new CLI-based implementation uses the standard `SubjectAccessReview` API (same as `kubectl auth can-i`) and supports anti-bypass checks (`allowed: false`) to prevent cluster-admin escalation.

## Adding a New Validation Type

To add a new validation type, touch **exactly these locations** — nothing else:

1. **`internal/validation/vtypes/types.go`**
   - Add a `TypeXxx ValidationType = "xxx"` constant
   - Add the `XxxSpec` and optional `XxxCheck` structs
   - Add an entry to `RegisteredTypes` (drives Zod schema generation automatically)

2. **`internal/validation/types.go`** — re-export the new constant and type alias (follow the existing pattern)

3. **`internal/validation/loader.go`** — add a `case TypeXxx:` in `parseSpec()` with field validation

4. **`internal/validation/executors/xxx/executor.go`** — create a new sub-package with:
   ```go
   package xxx
   func Execute(ctx context.Context, spec vtypes.XxxSpec, deps shared.Deps) (bool, string, error)
   ```

5. **`internal/validation/executor.go`** — add a `case TypeXxx:` in `Execute()` that calls `xxx.Execute(ctx, s, e.deps)`

6. **Tests**
   - Parsing tests in `loader_test.go`
   - Execution tests in `internal/validation/executors/xxx/executor_test.go`

### Zod Schema Generation

The CI workflow `generate-zod-schema.yaml` runs `go run hack/generate-schema/main.go` on every push to `main` and opens a PR in the `website` repo to update `schemas/challengeObjectives.ts`.

**No changes to `hack/generate-schema/main.go` are needed** — it reads `validation.RegisteredTypes` automatically. Adding the entry to `RegisteredTypes` is sufficient for the new type to appear in the generated TypeScript schema.

## Important Implementation Details

### Context Management

- **Always use**: `constants.KubeasyClusterContext` ("kind-kubeasy") when getting Kubernetes clients
- Namespace is set per-challenge in kubeconfig context
- `kube.SetNamespaceForContext()` updates namespace without changing context

### Infrastructure Deployment

- **Direct deployment**: Kyverno and local-path-provisioner are installed by downloading official install manifests and applying via `kube.ApplyManifest()`
- **Challenge deployment**: Challenges are distributed as OCI artifacts via `ghcr.io/kubeasy-dev/challenges/<slug>:latest` and pulled using `oras-go`
- Version constants are managed by Renovate using custom regex managers (see `renovate.json`)

### Error Handling

- Commands use `getChallenge(slug)` for consistent error handling
- API errors suggest running `kubeasy login` when authentication fails
- Logging via `logger` package writes to file when `--debug` is enabled

### Dependencies

- CI workflows use [Taskfile](https://taskfile.dev) for task automation
- Task is installed via `go-task/setup-task@v1` action in CI
- Tools are installed on-demand with status checks (skip if already installed)

## Release Process

- Triggered by pushing tags
- Uses GoReleaser for multi-platform builds
- Publishes to:
  - GitHub Releases (binaries + checksums)
  - NPM (via `npm publish`)
  - Cloudflare R2 (AWS S3-compatible storage)
- Go version: 1.25.4 (specified in go.mod and CI)

## Related Repositories

- **challenges** - Repository containing all challenge definitions with validation specs
- **website** - Next.js frontend for browsing challenges and tracking progress
- **documentation** - Fumadocs documentation site (user guides, developer docs)
