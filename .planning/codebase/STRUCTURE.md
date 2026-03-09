# Codebase Structure

**Analysis Date:** 2026-03-09

## Directory Layout

```
kubeasy-cli/
├── main.go                    # Binary entry point, calls cmd.Execute()
├── cmd/                       # Cobra command definitions (one file per command)
├── internal/                  # Private application packages
│   ├── api/                   # Backend HTTP client (hand-written wrapper)
│   ├── apigen/                # OpenAPI-generated HTTP client (DO NOT edit)
│   ├── constants/             # Global constants and version helpers
│   ├── demo/                  # Demo mode config and validator
│   ├── deployer/              # OCI artifact deployment and infra setup
│   ├── devutils/              # Developer tooling helpers (linting, display)
│   ├── keystore/              # Cross-platform credential storage
│   ├── kube/                  # Kubernetes client factory and common ops
│   ├── logger/                # File-backed leveled logger
│   ├── ui/                    # Terminal UI (pterm wrappers)
│   └── validation/            # Validation loader, executor, field path parser
│       └── fieldpath/         # Field path tokenizer/parser sub-package
├── test/                      # Test infrastructure
│   ├── e2e/                   # End-to-end workflow tests
│   ├── fixtures/              # YAML fixtures for validation tests
│   │   └── challenge-configs/ # Per-type validation example YAMLs
│   ├── helpers/               # Test helpers (envtest setup)
│   └── integration/           # Integration tests (require envtest cluster)
├── docs/                      # Developer documentation
├── hack/                      # Build and release helper scripts
├── scripts/                   # Shell scripts (CI, install helpers)
├── .github/                   # GitHub Actions workflows
├── openapi.json               # OpenAPI spec (source for apigen codegen)
├── oapi-codegen.yaml          # oapi-codegen config for client generation
├── Taskfile.yml               # Task automation (build, test, lint)
├── go.mod                     # Go module definition
└── go.sum                     # Dependency checksums
```

## Directory Purposes

**`cmd/`:**
- Purpose: All user-visible CLI commands using Cobra
- Contains: One `.go` file per command group; `common.go` for shared helpers
- Key files:
  - `cmd/root.go` - Root command, logger init, `--no-spinner` flag, CI mode detection
  - `cmd/challenge.go` - Parent `challenge` subcommand (no logic, just grouping)
  - `cmd/start.go` - `kubeasy challenge start`
  - `cmd/submit.go` - `kubeasy challenge submit`
  - `cmd/reset.go` - `kubeasy challenge reset`
  - `cmd/clean.go` - `kubeasy challenge clean`
  - `cmd/get.go` - `kubeasy challenge get`
  - `cmd/setup.go` - `kubeasy setup` (cluster creation + infra install)
  - `cmd/login.go` - `kubeasy login`
  - `cmd/common.go` - `getChallenge()`, `validateChallengeSlug()`, `deleteChallengeResources()`
  - `cmd/demo.go`, `cmd/demo_*.go` - Demo mode commands
  - `cmd/dev.go`, `cmd/dev_*.go` - Developer/authoring commands (`kubeasy dev`)

**`internal/api/`:**
- Purpose: Stable API layer over generated client; exposes named Go types
- Key files:
  - `auth.go` - `NewAuthenticatedClient()`, `NewPublicClient()`, token injection
  - `client.go` - Named operations: `GetChallenge`, `StartChallenge`, `SendSubmit`, `ResetChallenge`, etc.
  - `types.go` - Response types (`ChallengeEntity`, `ObjectiveResult`, `ChallengeSubmitResponse`, etc.)
  - `demo.go` - Demo-specific API operations

**`internal/apigen/`:**
- Purpose: Auto-generated OpenAPI client (generated from `openapi.json` via oapi-codegen)
- Key files: `client.gen.go` - all generated; never edit manually
- Regenerate with: `task generate` or `oapi-codegen -config oapi-codegen.yaml openapi.json`

**`internal/constants/`:**
- Purpose: Global application constants
- Key file: `const.go`
  - `WebsiteURL` - Backend API base URL (default: `http://localhost:3000`)
  - `KubeasyClusterContext = "kind-kubeasy"` - Always-used kube context
  - `KindNodeImage` - Kind node image (Renovate-managed via regex comment)
  - `KeyringServiceName = "kubeasy-cli"` - Keyring service identifier
  - `LogFilePath = "kubeasy-cli.log"` - Log file path

**`internal/deployer/`:**
- Purpose: Challenge OCI deployment and cluster infrastructure installation
- Key files:
  - `challenge.go` - `DeployChallenge()`, OCI pull via oras-go, manifest apply
  - `infrastructure.go` - `SetupInfrastructure()`, `IsInfrastructureReady()`, Kyverno + local-path-provisioner install
  - `cleanup.go` - `CleanupChallenge()` - namespace deletion, context restore
  - `local.go` - Local deployment path (non-OCI, for dev use)
  - `const.go` - `ChallengesOCIRegistry`, `KyvernoVersion`, `LocalPathProvisionerVersion` (all Renovate-managed)

**`internal/devutils/`:**
- Purpose: Developer tooling helpers for challenge authoring (`kubeasy dev` commands)
- Key files:
  - `display.go` - `DisplayValidationResults()` shared renderer
  - `lint.go` - Challenge YAML linting
  - `resolve.go` - Challenge resolution helpers
  - `slug.go` - Slug validation utilities
  - `watch.go` - File watch for live validation re-runs
  - `json_output.go` - JSON output for automation

**`internal/keystore/`:**
- Purpose: Cross-platform API key storage with fallback chain
- Fallback order: `KUBEASY_API_KEY` env var → system keyring → `~/.config/kubeasy-cli/credentials`
- Key files:
  - `keystore.go` - `Get()`, `Set()`, `Delete()`, `GetStorageType()`
  - `keystore_unix.go` - Unix file permission restriction (0600/0700)
  - `keystore_windows.go` - Windows ACL restriction via security APIs

**`internal/kube/`:**
- Purpose: Kubernetes client factory and common cluster operations
- Key files:
  - `client.go` - `GetKubernetesClient()`, `GetDynamicClient()`, `GetRestConfig()`, namespace CRUD, `WaitForDeploymentsReady()`, `WaitForStatefulSetsReady()`
  - `config.go` - `SetNamespaceForContext()` - kubeconfig context namespace manipulation
  - `manifest.go` - `ApplyManifest()`, `FetchManifest()` - manifest fetch from URL and server-side apply

**`internal/logger/`:**
- Purpose: File-backed leveled logger with line-count rotation
- Key file: `logger.go` - singleton logger, package-level `Debug()`, `Info()`, `Warning()`, `Error()` functions; klog integration

**`internal/ui/`:**
- Purpose: Terminal output abstraction (CI-mode aware)
- Key file: `ui.go` - `WaitMessage()`, `TimedSpinner()`, `Success()`, `Error()`, `ValidationResult()`, table/panel/prompt helpers using pterm

**`internal/validation/`:**
- Purpose: Validation definition loading and execution
- Key files:
  - `executor.go` - `Executor` struct with `Execute()`, `ExecuteAll()` (parallel), `ExecuteSequential()` (ordered/fail-fast)
  - `fieldvalidation.go` - `ValidateFieldPath()` compile-time field path checker using reflection against Kubernetes status types
  - `fieldpath/` - sub-package: field path tokenizer supporting `field`, `[index]`, `[field=value]` syntax

**`test/`:**
- Purpose: All test infrastructure outside of `_test.go` files in packages
- Key locations:
  - `test/fixtures/challenge-configs/` - example validation YAML for each type (used in unit/integration tests)
  - `test/helpers/envtest.go` - envtest cluster setup for integration tests
  - `test/integration/` - integration tests requiring a real API server (run via `task test:integration`)
  - `test/e2e/` - end-to-end workflow tests

## Key File Locations

**Entry Points:**
- `main.go` - Binary entry; only imports `cmd`
- `cmd/root.go` - Root Cobra command; logger and UI initialization

**Configuration:**
- `internal/constants/const.go` - All application-wide constants
- `internal/deployer/const.go` - Deployer-specific versions (OCI registry, Kyverno, provisioner)
- `openapi.json` - Backend API contract (source of truth for `apigen`)
- `oapi-codegen.yaml` - Code generation config

**Core Logic:**
- `internal/validation/executor.go` - All 5 validation type implementations
- `internal/deployer/challenge.go` - OCI artifact pull and cluster apply
- `internal/api/client.go` - All named backend operations

**Testing:**
- `test/fixtures/challenge-configs/` - YAML fixtures for validation tests
- `test/helpers/envtest.go` - Shared envtest cluster lifecycle

## Naming Conventions

**Files:**
- Commands: match the command name (`start.go`, `submit.go`)
- Dev commands: prefixed with `dev_` (`dev_apply.go`, `dev_validate.go`)
- Demo commands: prefixed with `demo_` (`demo_start.go`, `demo_submit.go`)
- Generated: suffixed with `.gen.go` (`client.gen.go`)
- Platform-specific: suffixed with `_unix.go` / `_windows.go`
- Tests: suffixed with `_test.go` (co-located in same package)

**Packages:**
- Match directory name (snake_case directories, lowercase package names)
- Commands all use `package cmd`
- No internal sub-package nesting except `validation/fieldpath`

**Functions:**
- Exported: PascalCase
- Internal helpers: camelCase
- Constructor pattern: `New<Type>(...)` (e.g., `NewExecutor`, `NewAuthenticatedClient`)

**Variables:**
- Package-level vars for Cobra commands: `<name>Cmd` (e.g., `startChallengeCmd`, `submitCmd`)
- Constants file uses `var` (not `const`) for Renovate-managed values

## Where to Add New Code

**New top-level command:**
- Create `cmd/<name>.go` with a `var <name>Cmd` and `init()` that calls `rootCmd.AddCommand(<name>Cmd)`

**New challenge subcommand:**
- Create `cmd/<name>.go` with `init()` that calls `challengeCmd.AddCommand(<name>Cmd)`

**New validation type:**
- Add type constant to `internal/validation/types.go`
- Add spec struct to `internal/validation/types.go`
- Add `case TypeXxx:` to `executor.go` `Execute()` switch
- Add `executeXxx()` method on `Executor` in `executor.go`
- Add fixture YAML to `test/fixtures/challenge-configs/`
- Add integration test in `test/integration/`

**New backend API call:**
- Add to `openapi.json` first, then regenerate `internal/apigen/client.gen.go` with oapi-codegen
- Add named wrapper function in `internal/api/client.go` using generated method
- Add response type to `internal/api/types.go` if needed

**New Kubernetes helper:**
- Add to `internal/kube/client.go` (for client/resource operations) or `internal/kube/manifest.go` (for manifest ops)

**Shared command helpers:**
- Add to `cmd/common.go`

**Developer tooling:**
- Add to `internal/devutils/` with corresponding `cmd/dev_<name>.go` command

**Constants:**
- Application-wide: `internal/constants/const.go`
- Deployer-specific versions: `internal/deployer/const.go` (include Renovate annotation comment if version should be auto-updated)

## Special Directories

**`.planning/`:**
- Purpose: GSD planning documents (phases, codebase analysis)
- Generated: No
- Committed: Yes

**`.github/`:**
- Purpose: GitHub Actions CI workflows and release automation (GoReleaser)
- Generated: No
- Committed: Yes

**`hack/`:**
- Purpose: Build and tooling helper scripts
- Generated: No
- Committed: Yes

**`node_modules/`:**
- Purpose: npm dependencies for the npm-based install script (`install.js`)
- Generated: Yes (via npm install)
- Committed: No (but `package-lock.json` is committed)

**`bin/`:**
- Purpose: Locally built binary output from `task build`
- Generated: Yes
- Committed: No

**`logs/`:**
- Purpose: Log file output directory (if configured)
- Generated: Yes at runtime
- Committed: No

---

*Structure analysis: 2026-03-09*
