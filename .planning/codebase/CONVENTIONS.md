# Coding Conventions

**Analysis Date:** 2026-03-09

## Naming Patterns

**Files:**
- Snake_case for multi-word files: `challenge_test.go`, `infrastructure_test.go`
- Command files mirror their command name: `start.go`, `submit.go`, `reset.go`
- Sub-command files prefixed with parent: `dev_apply.go`, `dev_create.go`, `demo_start.go`
- Test files co-located with source: `challenge.go` → `challenge_test.go`
- Generated files suffixed with `.gen.go`: `internal/apigen/client.gen.go`

**Functions:**
- PascalCase for exported: `DeployChallenge`, `NewExecutor`, `LoadForChallenge`
- camelCase for unexported: `getChallenge`, `parseSpec`, `validateTarget`, `loadFromURL`
- Constructor pattern: `NewExecutor(...)`, `NewAuthenticatedClient()`, `NewPublicClient()`
- Injected-client variants for testability: `IsInfrastructureReadyWithClient(ctx, clientset)` alongside `IsInfrastructureReady(ctx)`

**Variables:**
- camelCase for local: `challengeSlug`, `restConfig`, `dynamicClient`
- PascalCase for package-level exports: `KubeasyClusterContext`, `KeyringServiceName`
- Var (not const) for Renovate-managed versions to allow regex substitution: `var KindNodeImage = "kindest/node:v1.35.0"`

**Types:**
- PascalCase structs: `Executor`, `ValidationConfig`, `TestEnvironment`, `ChallengeEntity`
- Type aliases as named strings: `type ValidationType string`, `type LogLevel int`
- Constants in typed groups with iota or explicit string values:
  ```go
  const (
      TypeStatus      ValidationType = "status"
      TypeCondition   ValidationType = "condition"
      TypeLog         ValidationType = "log"
      TypeEvent       ValidationType = "event"
      TypeConnectivity ValidationType = "connectivity"
  )
  ```

**Constants:**
- Prefixed with their category in const blocks: `errNoMatchingPods`, `msgAllStatusChecksPassed`
- Error message constants prefixed `err`, success message constants prefixed `msg`

## Code Style

**Formatting:**
- Tool: `gofmt` (enforced via golangci-lint formatter `gofmt`)
- Imports also managed by `goimports` formatter
- Run with: `task fmt`

**Linting:**
- Tool: `golangci-lint` v2, config at `.github/linters/.golangci.yml`
- Active linters: `errcheck`, `goconst`, `gocritic`, `gosec`, `govet`, `ineffassign`, `misspell`, `nolintlint`, `staticcheck`, `unconvert`, `unused`
- `nolint` directives must include a reason comment: `//nolint:gosec // URL validated against ChallengesRepoBaseURL`

## Import Organization

**Order (goimports enforces):**
1. Standard library
2. External packages
3. Internal packages (`github.com/kubeasy-dev/kubeasy-cli/internal/...`)

**Example from `internal/validation/executor.go`:**
```go
import (
    "bytes"
    "context"
    "fmt"

    "github.com/kubeasy-dev/kubeasy-cli/internal/logger"
    "github.com/kubeasy-dev/kubeasy-cli/internal/validation/fieldpath"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)
```

**Kubernetes package aliasing:**
- Always alias Kubernetes API packages to avoid name collisions: `corev1`, `appsv1`, `metav1`, `apierrors`

**Path Aliases:**
- None. All imports use full module paths.

## Error Handling

**Primary pattern:** Wrap errors with context using `fmt.Errorf("description: %w", err)`

```go
// Always wrap with context
if err := kube.ApplyManifest(ctx, data, slug, mapper, dynamicClient); err != nil {
    return fmt.Errorf("failed to apply manifest %s: %w", filepath.Base(f), err)
}
```

**Command layer (cmd/):** Display user-facing error then return wrapped error
```go
if err != nil {
    ui.Error("Failed to get Kubernetes client")
    return fmt.Errorf("failed to get Kubernetes client: %w", err)
}
```

**Fire-and-forget pattern for non-critical operations:**
```go
func TrackSetup() {
    client, err := NewAuthenticatedClient()
    if err != nil {
        logger.Debug("Failed to create client for tracking: %v", err)
        return  // silently ignore
    }
}
```

**Defer for cleanup with blank error discard when not critical:**
```go
defer func() { _ = resp.Body.Close() }()
defer os.RemoveAll(tmpDir)
```

**Idiomatic nil checks before using optional responses:**
```go
if resp.JSON200 == nil {
    return nil, parseErrorResponse(resp.HTTPResponse, resp.Body)
}
```

## Logging

**Framework:** Custom `logger` package at `internal/logger/logger.go`, wraps `k8s.io/klog/v2`

**Levels:** `DEBUG`, `INFO`, `WARNING`, `ERROR` (via `logger.Debug(...)`, `logger.Info(...)`, `logger.Warning(...)`)

**Patterns:**
- Use `logger.Debug` for internal state and intermediate steps
- Use `logger.Info` for user-visible progress in backend (deployer, validator)
- Use `ui.*` functions for end-user display in `cmd/` layer (never `logger.*` for user output)
- Format strings like `logger.Info("Deploying challenge '%s' from OCI registry...", slug)`

## Comments

**When to Comment:**
- Exported types and functions: always document with godoc-style comment starting with the symbol name
- Non-obvious decisions: explain the WHY, not the WHAT
- Security linting suppressions: always include reason after `//nolint:` directive

**Package doc comments:**
```go
// Package validation provides types and executors for CLI-based validation
// of Kubernetes resources.
package validation
```

**Renovate annotation format (required for version management):**
```go
// renovate: datasource=docker depName=kindest/node
var KindNodeImage = "kindest/node:v1.35.0"
```

## Function Design

**Size:** Functions are kept focused. Large switch statements are acceptable when routing to type-specific handlers.

**Parameters:** Dependency injection preferred — constructors accept interfaces (`kubernetes.Interface`, `dynamic.Interface`) not concrete types, enabling fake clients in tests.

**Return Values:**
- Single return: `error` for operations, value for queries
- Multiple return: `(value, error)` — value first, error last (Go idiom)
- Optional/nullable values use pointer: `*string`, `*LoginResponse`

**Context handling:** All functions with I/O accept `context.Context` as first parameter.

## Module Design

**Exports:**
- Only export what external packages need. Internal helpers stay unexported.
- Backward-compat wrappers preserved and named clearly: `GetChallenge` wraps `GetChallengeBySlug`, `StartChallenge` wraps `StartChallengeWithResponse`

**Package structure:** One package per directory, package name matches directory name.

**Testability pattern:** Functions with external dependencies provide two versions:
- `IsInfrastructureReady(ctx)` — production, creates own client
- `IsInfrastructureReadyWithClient(ctx, clientset)` — testable, accepts injected client

**Constants package (`internal/constants/const.go`):**
- Runtime-overridable values declared as `var` (not `const`) so ldflags can inject at build time and tests can override
- Build-time injection via ldflags: `Version`, `LogFilePath`, `WebsiteURL`, `ExercicesRepoBranch`

---

*Convention analysis: 2026-03-09*
