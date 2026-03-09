# Architecture

**Analysis Date:** 2026-03-09

## Pattern Overview

**Overall:** Layered CLI application with a command/service separation

**Key Characteristics:**
- Commands in `cmd/` handle UX (spinners, prompts, validation) and orchestrate internal packages
- Internal packages in `internal/` contain all business logic and are never imported by each other except through well-defined dependency directions
- No global state between commands; each command resolves its own Kubernetes clients
- Generated HTTP client (`internal/apigen/`) is wrapped by a hand-written API layer (`internal/api/`) that exposes stable Go types

## Layers

**Command Layer:**
- Purpose: User-facing CLI commands, argument parsing, spinner/UI orchestration
- Location: `cmd/`
- Contains: One file per command, shared helpers in `cmd/common.go`
- Depends on: `internal/api`, `internal/deployer`, `internal/kube`, `internal/validation`, `internal/ui`, `internal/keystore`, `internal/constants`, `internal/logger`
- Used by: `main.go` via `cmd.Execute()`

**API Client Layer:**
- Purpose: HTTP communication with the Kubeasy backend (Next.js website)
- Location: `internal/api/`
- Contains: `auth.go` (token injection), `client.go` (named operations), `types.go` (stable response types), `demo.go` (unauthenticated operations)
- Depends on: `internal/apigen` (generated), `internal/keystore`, `internal/constants`, `internal/logger`
- Used by: `cmd/`

**Generated Client Layer:**
- Purpose: OpenAPI-generated HTTP client from `openapi.json`
- Location: `internal/apigen/client.gen.go`
- Contains: Auto-generated structs and HTTP methods
- Depends on: nothing internal
- Used by: `internal/api/` only (acts as a stable boundary)

**Deployer Layer:**
- Purpose: Cluster infrastructure setup and challenge OCI artifact deployment
- Location: `internal/deployer/`
- Contains: `challenge.go` (OCI pull + apply), `infrastructure.go` (Kyverno + provisioner install), `cleanup.go`, `local.go`, `image.go`, `const.go`
- Depends on: `internal/kube`, `internal/logger`
- Used by: `cmd/`

**Validation Layer:**
- Purpose: Load and execute challenge validation specs against the live cluster
- Location: `internal/validation/`
- Contains: `executor.go` (5 validation type implementations), `fieldvalidation.go` (compile-time field path validation), sub-package `internal/validation/fieldpath/` (field path parser)
- Depends on: `internal/logger`, Kubernetes client-go, `internal/validation/fieldpath`
- Used by: `cmd/`, `internal/devutils/`

**Kubernetes Abstraction Layer:**
- Purpose: Kubernetes client creation and common operations (namespace CRUD, manifest apply, readiness waits)
- Location: `internal/kube/`
- Contains: `client.go` (clientset, dynamic client, REST config), `config.go` (kubeconfig manipulation), `manifest.go` (apply/fetch manifests)
- Depends on: `internal/constants`, `internal/logger`, k8s.io/client-go
- Used by: `internal/deployer`, `cmd/`

**Keystore Layer:**
- Purpose: Cross-platform credential storage with multi-backend fallback
- Location: `internal/keystore/`
- Contains: `keystore.go` (Get/Set/Delete with fallback chain), `keystore_unix.go`, `keystore_windows.go` (platform-specific file permissions)
- Depends on: `internal/constants`, `internal/logger`, `github.com/zalando/go-keyring`
- Used by: `internal/api/auth.go`, `cmd/login.go`, `cmd/setup.go`

**UI Layer:**
- Purpose: Terminal output formatting (spinners, progress, tables, prompts)
- Location: `internal/ui/ui.go`
- Contains: CI-mode aware spinners (`WaitMessage`, `TimedSpinner`), pterm wrappers, `ValidationResult` renderer
- Depends on: `github.com/pterm/pterm`
- Used by: `cmd/`

**Dev Utilities Layer:**
- Purpose: Developer tooling (challenge linting, local validation, display helpers)
- Location: `internal/devutils/`
- Contains: `display.go`, `lint.go`, `resolve.go`, `slug.go`, `watch.go`, `json_output.go`
- Depends on: `internal/validation`, `internal/ui`
- Used by: `cmd/dev_*.go` commands

**Constants & Logger:**
- Location: `internal/constants/const.go`, `internal/logger/logger.go`
- Purpose: Global configuration values and file-backed leveled logger (DEBUG/INFO/WARN/ERROR)
- Logger writes only to file (`kubeasy-cli.log`), never stdout; klog is redirected to the same file

## Data Flow

**Challenge Start Flow:**
1. `cmd/start.go` receives slug, calls `api.GetChallenge(slug)` and `api.GetChallengeProgress(slug)`
2. `kube.GetDynamicClient()` and `kube.GetKubernetesClient()` resolve clients from `kind-kubeasy` context in `~/.kube/config`
3. `kube.CreateNamespace(ctx, clientset, slug)` creates the challenge namespace
4. `deployer.DeployChallenge(ctx, clientset, dynamicClient, slug)` pulls OCI artifact from `ghcr.io/kubeasy-dev/challenges/<slug>:latest` via oras-go, applies `manifests/` and `policies/` YAML files
5. `deployer.WaitForChallengeReady(ctx, clientset, slug)` polls Deployments and StatefulSets until ready
6. `api.StartChallenge(slug)` registers progress on backend

**Challenge Submit Flow:**
1. `cmd/submit.go` verifies challenge and progress via `api`
2. `validation.LoadForChallenge(slug)` fetches `challenge.yaml` (local file first, then GitHub fallback)
3. `validation.NewExecutor(clientset, dynamicClient, restConfig, namespace)` creates executor scoped to challenge namespace
4. `executor.ExecuteAll(ctx, config.Validations)` runs all validations in parallel via goroutines
5. Results converted to `[]api.ObjectiveResult`, sent via `api.SendSubmit(slug, results)`
6. UI renders results grouped by validation type

**Authentication Flow:**
1. `keystore.Get()` checks: `KUBEASY_API_KEY` env var → system keyring → `~/.config/kubeasy-cli/credentials` file
2. `api.NewAuthenticatedClient()` creates apigen client with `BearerAuthEditorFn` that injects token per request
3. All API calls use `context.Background()` with 30-second HTTP timeout

**State Management:**
- No in-memory application state between commands
- Challenge progress state lives entirely in the backend API
- Kubernetes cluster is the source of truth for deployed resource state
- Credentials persisted externally (keyring or file)

## Key Abstractions

**ValidationConfig and Validation:**
- Purpose: Typed representation of `challenge.yaml` validation specs
- Files: `internal/validation/types.go` (types), `internal/validation/loader.go` (parsing)
- Pattern: `Validation.Spec` is an `interface{}` that holds one of `StatusSpec`, `ConditionSpec`, `LogSpec`, `EventSpec`, or `ConnectivitySpec`; type-switched in `executor.go`

**Executor:**
- Purpose: Stateful Kubernetes validation runner scoped to a namespace
- Files: `internal/validation/executor.go`
- Pattern: Constructor injection of `clientset`, `dynamicClient`, `restConfig`, `namespace`; `ExecuteAll` runs goroutines for parallel execution; `ExecuteSequential` with optional fail-fast for ordered execution

**apigen Client:**
- Purpose: Generated OpenAPI client, acts as a schema contract between CLI and backend
- Files: `internal/apigen/client.gen.go`
- Pattern: Generated from `openapi.json` via oapi-codegen; all hand-written code in `internal/api/` wraps this and re-exposes stable named types

## Entry Points

**Binary:**
- Location: `main.go`
- Triggers: OS process start
- Responsibilities: Delegates immediately to `cmd.Execute()`

**Root Command:**
- Location: `cmd/root.go`
- Triggers: Every command invocation via `PersistentPreRun`
- Responsibilities: Initializes logger to `constants.LogFilePath`, detects TTY and sets CI mode for spinners

**Setup Command:**
- Location: `cmd/setup.go`
- Triggers: `kubeasy setup`
- Responsibilities: Creates Kind cluster named `kubeasy` with `kind-kubeasy` context, installs Kyverno + local-path-provisioner

**Challenge Subcommands:**
- Location: `cmd/start.go`, `cmd/submit.go`, `cmd/reset.go`, `cmd/clean.go`, `cmd/get.go`
- Triggers: `kubeasy challenge <subcommand>`
- Responsibilities: Full challenge lifecycle management

## Error Handling

**Strategy:** Errors wrapped with `fmt.Errorf("context: %w", err)` at each layer boundary, surfaced to user via `ui.Error()` in commands.

**Patterns:**
- Commands return `error` from `RunE`; cobra prints it and exits with code 1
- `getChallenge(slug)` in `cmd/common.go` is a shared helper for validated challenge fetching used by reset, clean, get
- Kubernetes API errors: `apierrors.IsNotFound(err)` checked before generic error handling
- Validation failures: `Result.Passed = false` with descriptive `Result.Message`; never panic

## Cross-Cutting Concerns

**Logging:** File-only logging to `kubeasy-cli.log` (same directory as binary). Levels: DEBUG/INFO/WARN/ERROR. Initialized in `cmd/root.go` `PersistentPreRun`. klog redirected to same file. Log rotation keeps newest 50% when 1000-line limit is hit.

**Validation:** Challenge slug validated by regex `^[a-z0-9]+(-[a-z0-9]+)*$` in `cmd/common.go` before any API calls.

**Authentication:** All authenticated API calls go through `api.NewAuthenticatedClient()` which injects Bearer token via `BearerAuthEditorFn` request editor. Public endpoints (challenge types, themes, difficulties) use `api.NewPublicClient()`.

**Kubernetes Context:** All kube clients are hardcoded to `constants.KubeasyClusterContext = "kind-kubeasy"`. Never uses current context.

---

*Architecture analysis: 2026-03-09*
