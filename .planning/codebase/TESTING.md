# Testing Patterns

**Analysis Date:** 2026-03-09

## Test Framework

**Runner:**
- Go's built-in `testing` package
- No separate test runner binary

**Assertion Library:**
- `github.com/stretchr/testify/assert` — non-fatal assertions
- `github.com/stretchr/testify/require` — fatal assertions (stops test on failure)

**Kubernetes Fake Clients:**
- `k8s.io/client-go/kubernetes/fake` — fake typed clientset
- `k8s.io/client-go/dynamic/fake` — fake dynamic client
- `sigs.k8s.io/controller-runtime/pkg/envtest` — real API server for integration tests

**Run Commands:**
```bash
task test              # Run all tests (unit + integration)
task test:unit         # Unit tests only (no build tags)
task test:integration  # Integration tests (requires envtest assets: task setup:envtest)
task test:e2e          # Kind e2e tests (requires Docker)
task test:coverage     # Generate merged HTML coverage report
```

**Flags used in CI:**
```bash
go test -v -race -coverprofile=coverage-unit.out -covermode=atomic \
  -coverpkg=./internal/... $(go list ./... | grep -v /test/integration)
```

## Test File Organization

**Location:**
- Unit tests: co-located with source file in same package directory
  - `internal/validation/executor.go` → `internal/validation/executor_test.go`
  - `internal/deployer/infrastructure.go` → `internal/deployer/infrastructure_test.go`
  - `internal/api/client.go` → `internal/api/client_test.go`, `internal/api/client_http_test.go`
- Integration tests: `test/integration/` (separate package `package integration`)
- E2E tests: `test/e2e/` (separate package `package e2e`)
- Shared test helpers: `test/helpers/` (package `helpers`)

**Naming:**
- Unit test files: `<source_file>_test.go`
- HTTP integration files: `<source_file>_http_test.go` (for httptest server tests)
- Integration test files: `<feature>_test.go` under `test/integration/`

**Build tags:**
- Integration tests: `//go:build integration` + `// +build integration`
- E2E tests: `//go:build kindintegration` + `// +build kindintegration`
- Unit tests: no build tag (run by default)

**Package declarations:**
- Co-located tests use the same package (whitebox): `package validation`
- Integration/e2e tests use separate packages (blackbox): `package integration`, `package e2e`

## Test Structure

**Unit test suite organization (flat functions):**
```go
func TestExecuteAll(t *testing.T) {
    // setup
    clientset := fake.NewClientset(pod)
    executor := NewExecutor(clientset, ...)

    // act
    results := executor.ExecuteAll(context.Background(), validations)

    // assert
    assert.True(t, results[0].Passed)
}
```

**Subtests with `t.Run`:**
```go
func TestIsInfrastructureReady_PartiallyReady(t *testing.T) {
    clientset := fake.NewClientset(...)
    ready, err := IsInfrastructureReadyWithClient(context.Background(), clientset)
    require.NoError(t, err)
    assert.False(t, ready, "should be false when local-path-provisioner is not ready")
}

// Or grouped:
func TestCreateNamespace_Logic(t *testing.T) {
    t.Run("creates new namespace successfully", func(t *testing.T) { ... })
    t.Run("idempotent - namespace already exists", func(t *testing.T) { ... })
}
```

**Patterns:**
- Use `require.NoError` for setup steps that must succeed for the test to be meaningful
- Use `assert.*` for the actual assertions being tested
- Always provide a message on assertions: `assert.False(t, ready, "should be false when ...")`
- Use `t.Helper()` on all shared helper functions

## Mocking

**Kubernetes clients:** Use fake clients from `k8s.io/client-go/kubernetes/fake` and `k8s.io/client-go/dynamic/fake`:
```go
clientset := fake.NewClientset(pod, deployment)
dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
executor := NewExecutor(clientset, dynamicClient, &rest.Config{}, "test-ns")
```

**HTTP servers:** Use `net/http/httptest` for API client tests:
```go
server := setupMockServer(t, func(w http.ResponseWriter, r *http.Request) {
    assert.Equal(t, "GET", r.Method)
    assert.Equal(t, "/api/cli/user", r.URL.Path)
    assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(response)
})
defer server.Close()
defer overrideServerURL(t, server.URL)()
```

**Keyring:** Use `keyring.MockInit()` from `github.com/zalando/go-keyring`:
```go
func setupKeyring(t *testing.T, token string) {
    t.Helper()
    keyring.MockInit()
    err := keyring.Set(constants.KeyringServiceName, "api_key", token)
    require.NoError(t, err)
}
```

**URL override:** Mutate package-level `var` for test isolation, restore with deferred cleanup:
```go
func overrideServerURL(t *testing.T, serverURL string) func() {
    t.Helper()
    oldWebsiteURL := constants.WebsiteURL
    constants.WebsiteURL = serverURL
    return func() {
        constants.WebsiteURL = oldWebsiteURL
    }
}
// Usage:
defer overrideServerURL(t, server.URL)()
```

**What to Mock:**
- External HTTP calls: always use httptest.Server
- Kubernetes API: always use fake clientset
- Keyring: always use `keyring.MockInit()`
- File system state: use temp dirs with `t.TempDir()`

**What NOT to Mock:**
- Pure logic functions (parsing, validation, formatting)
- Functions that accept injected interfaces — pass fake directly

## Fixtures and Factories

**Object factories (in test files, not shared):**
```go
func makeDeployment(namespace, name string, replicas int32, ready bool) *appsv1.Deployment {
    dep := &appsv1.Deployment{
        ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
        Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
        Status:     appsv1.DeploymentStatus{Replicas: replicas},
    }
    if ready {
        dep.Status.ReadyReplicas = replicas
    }
    return dep
}

func makeNamespace(name string) *corev1.Namespace {
    return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
}
```

**Test helpers struct (shared, `test/helpers/envtest.go`):**
```go
type TestEnvironment struct {
    Env           *envtest.Environment
    Config        *rest.Config
    Clientset     *kubernetes.Clientset
    DynamicClient dynamic.Interface
    Namespace     string
}
// Usage in integration tests:
env := helpers.SetupEnvTest(t)
// env.Namespace is auto-derived from t.Name(), sanitized for K8s
// Cleanup registered automatically via t.Cleanup()
```

**Fixture files:**
- Challenge config YAML fixtures: `test/fixtures/challenge-configs/`

**String pointer helper (found in `internal/api/` tests):**
```go
func strPtr(s string) *string { return &s }
```

## Coverage

**Requirements:** No enforced minimum, but `task test:coverage` outputs final percentage.

**View Coverage:**
```bash
task test:coverage        # Generates coverage.html and prints summary
go tool cover -func=coverage.out | tail -1   # Quick summary
```

**Coverage files:**
- Unit: `coverage-unit.out`
- Integration: `coverage-integration.out`
- Merged: `coverage.out` + `coverage.html`
- Merged using: `gocovmerge coverage-unit.out coverage-integration.out > coverage.out`

**Coverage scope:** `-coverpkg=./internal/...` — only internal packages are measured.

## Test Types

**Unit Tests:**
- Scope: single package, no external dependencies
- Fake K8s clients for all K8s interactions
- httptest.Server for all HTTP interactions
- Files: co-located `*_test.go` in `internal/` subdirs
- Invocation: `go test ./...` (no build tag)

**Integration Tests:**
- Scope: real Kubernetes API server via `controller-runtime/envtest`
- Requires: `KUBEBUILDER_ASSETS` env var pointing to envtest binaries (set by `task setup:envtest`)
- Files: `test/integration/*_test.go` with `//go:build integration`
- Invocation: `go test -tags=integration ./test/integration/... -timeout 15m`
- Each test gets its own isolated namespace derived from `t.Name()`

**E2E Tests:**
- Scope: full Kind cluster lifecycle (real Docker containers)
- Requires: Docker running, Kind installed
- Files: `test/e2e/*_test.go` with `//go:build kindintegration`
- Cluster lifecycle managed in `TestMain`: creates cluster before tests, deletes after
- Invocation: `go test -tags=kindintegration ./test/e2e/... -timeout 10m`

## Common Patterns

**Async/polling in tests (integration only):**
```go
func (e *TestEnvironment) WaitForPod(podName string, timeout time.Duration) *corev1.Pod {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    ticker := time.NewTicker(50 * time.Millisecond)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            e.t.Fatalf("Timeout waiting for pod %s", podName)
        case <-ticker.C:
            pod, err := e.Clientset.CoreV1().Pods(e.Namespace).Get(...)
            if err == nil { return pod }
        }
    }
}
```

**Error Testing:**
```go
result := executor.Execute(context.Background(), validation)
assert.False(t, result.Passed)
assert.Contains(t, result.Message, "Unknown validation type")
assert.Equal(t, "test-key", result.Key)
```

**Testing K8s API errors explicitly:**
```go
_, err := clientset.CoreV1().Namespaces().Get(ctx, "test-namespace", metav1.GetOptions{})
assert.True(t, apierrors.IsNotFound(err), "namespace should not exist initially")
```

**YAML literal test input for parser tests:**
```go
func TestParse_StatusValidation(t *testing.T) {
    yaml := `
objectives:
  - key: deployment-ready
    type: status
    spec:
      target:
        kind: Deployment
        labelSelector:
          app: test-app
      checks:
        - field: readyReplicas
          operator: ">="
          value: 3
`
    config, err := Parse([]byte(yaml))
    require.NoError(t, err)
    require.Len(t, config.Validations, 1)
    ...
}
```

---

*Testing analysis: 2026-03-09*
