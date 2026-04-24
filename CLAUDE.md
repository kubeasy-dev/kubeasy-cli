# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`kubeasy-cli` is a command-line tool built with Go and Cobra that helps developers learn Kubernetes through practical challenges. It manages local Kind clusters, deploys challenges, and validates solutions using a **CLI-based validation system**.

### Architecture: API Hub

The CLI always talks to the Kubeasy API (`https://kubeasy.dev`), never directly to the registry. The API proxies challenge data from the registry service.

```
registry.kubeasy.dev  ←→  API (kubeasy.dev)  ←→  CLI
```

- Challenge YAML and manifests are fetched from the API (`GET /challenges/:slug/yaml`, `GET /challenges/:slug/manifests`)
- The CLI uses `registry/pkg/challenges` **only as a local Go parser** — no HTTP calls to the registry
- API URL is overridable via `KUBEASY_API_URL` env var (see `internal/constants/constants.go`)
- Dev mode (`kubeasy dev`) is filesystem-first: finds `challenge.yaml` locally, no network needed

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
    - `start.go` - Fetches manifests tar.gz from API, applies to cluster, tracks progress
    - `submit.go` - Validates solutions by loading validation specs and submitting results
    - `reset.go` - Deletes resources and resets progress in backend
    - `clean.go` - Removes challenge resources without resetting backend
    - `get.go` - Displays challenge details
  - `common.go` - Shared helper functions for commands

### Core Packages (internal/)

#### `internal/api/`

- Communicates with the Kubeasy API (`https://kubeasy.dev`, overridable via `KUBEASY_API_URL`)
- Uses a generated OpenAPI client (`internal/apigen/`) — do not hand-edit
- `auth.go` - `NewAuthenticatedClient()` / `NewPublicClient()` — injects Bearer token from keyring
- `client.go` - Higher-level wrappers: `GetChallengeBySlug`, `SubmitChallenge`, `Login`, `GetProfile`, etc.
- `types.go` - Named response types (stable interface over generated anonymous structs)

#### `internal/deployer/`

Handles direct deployment of infrastructure and challenges.

- `infrastructure.go` - Installs Kyverno and local-path-provisioner directly via HTTP manifests
  - `SetupInfrastructure()` - Downloads and applies install manifests, waits for readiness
  - `IsInfrastructureReady()` / `IsInfrastructureReadyWithClient(ctx, clientset)` - Readiness checks
- `challenge.go` - Deploys challenges by fetching manifests tar.gz from the API
  - `DeployChallenge(ctx, clientset, dynamicClient, slug)` - Fetches tar.gz, extracts, applies manifests, waits for ready
- `registry.go` - Low-level helpers for fetching manifests from a registry-compatible URL (used in dev mode)
- `cleanup.go` - `CleanupChallenge(ctx, clientset, slug)` - Deletes namespace and restores kubectl context

#### `internal/validation/`

CLI-based validation system — loads specs from challenge.yaml and executes checks against the cluster.

- `loader.go` - Loads validation configs
  - `LoadForChallenge(slug)` - Tries local file first (`FindLocalChallengeFile`), then API (`GET /challenges/:slug/yaml`)
  - `Parse(data []byte)` - Delegates to `registry/pkg/challenges.ParseBytes()`, applies CLI defaults
  - `fromObjective()` - Converts registry pointer types to CLI value types, applies SinceSeconds/Timeout defaults

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

#### `internal/constants/constants.go`

- Global constants:
  - `WebsiteURL = "https://kubeasy.dev"` — API base URL (override with `KUBEASY_API_URL` or `API_URL`)
  - `KubeasyClusterContext = "kind-kubeasy"`
  - `KeyringServiceName = "kubeasy-cli"`
  - `LogFilePath` - Path for debug logs
  - `KindNodeImage` - Kind node image (Renovate-managed)

#### `internal/logger/logger.go`

- Custom logging utility with file output support
- Levels: DEBUG, INFO, WARN, ERROR
- Controlled via `--debug` flag on root command

### Key Workflows

#### Challenge Lifecycle

1. **Setup**: `kubeasy setup` → Creates Kind cluster → Installs Kyverno + local-path-provisioner
2. **Start**: `kubeasy challenge start <slug>` → Creates namespace → Fetches manifests tar.gz from API → Applies manifests → Tracks progress
3. **Work**: User modifies cluster resources manually
4. **Submit**: `kubeasy challenge submit <slug>` → Loads validations from challenge.yaml → Executes checks → Sends results to API
5. **Clean/Reset**: `kubeasy challenge clean/reset <slug>` → Deletes namespace ± backend data

#### Authentication Flow

- User runs `kubeasy login` → Enters API key → Stored in system keyring
- All authenticated API calls retrieve the key from keyring and send it as a Bearer token
- Public endpoints (challenge list, YAML fetch) use `NewPublicClient()` — no auth required

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

`hack/generate-schema/main.go` generates `packages/api-schemas/src/objectives.ts` in the monorepo.
It is triggered by `generate-zod-schema.yaml` on every push to `main` that touches `vtypes/types.go`.

The generated file contains **two families of schemas**, both driven by `vtypes/types.go`:

**Objective/validation specs** — driven by `RegisteredTypes`:
- One Zod schema per spec type (`StatusSpecSchema`, `LogSpecSchema`, etc.)
- `ObjectiveTypeSchema` enum (all registered type names)
- `ObjectiveSpecSchema` union (all registered spec schemas)
- `ObjectiveSchema` (key, title, description, order, type, spec)

**Challenge YAML format** — driven by `ChallengeYamlSpec` + `ChallengeDifficultyValues` + `ChallengeTypeValues`:
- `ChallengeYamlDifficultySchema`, `ChallengeYamlTypeSchema` enums
- `ChallengeYamlSchema` (full challenge.yaml structure)

`challengeSyncSchema` in `apps/api/src/schemas/sync.ts` is **derived** from `ChallengeYamlSchema`:
```ts
ChallengeYamlSchema.omit({ minRequiredVersion: true }).extend({ slug: z.string() })
```

**No changes to `hack/generate-schema/main.go` are needed** when adding a new validation type — it reads `RegisteredTypes` automatically. Adding the entry to `RegisteredTypes` is sufficient.

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
