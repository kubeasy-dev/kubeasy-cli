# Kubeasy CLI Tests

This directory contains the test suite for the Kubeasy CLI, including unit tests and integration tests for the validation engine.

## Structure

```
test/
├── fixtures/                    # Test fixtures and sample data
│   └── challenge-configs/       # Sample challenge validation configs
├── helpers/                     # Test helpers and utilities
│   └── envtest.go              # EnvTest setup and K8s helpers
└── integration/                 # Integration tests
    ├── status_validation_test.go
    ├── event_validation_test.go
    └── metrics_validation_test.go
```

## Prerequisites

### EnvTest

Integration tests use [controller-runtime/envtest](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest) to run a local Kubernetes API server without requiring Docker or Kind.

To install envtest binaries (along with other dev tools):

```bash
make install-tools
```

Or to install only envtest:

```bash
make setup-envtest
```

This will download and install the Kubernetes control plane binaries (etcd, kube-apiserver, kubectl) to `./bin/k8s/`.

## Running Tests

### All Tests

Run all unit and integration tests:

```bash
make test
# or
make test-all
```

### Unit Tests Only

Run only unit tests (fast, no K8s cluster needed):

```bash
make test-unit
```

### Integration Tests Only

Run only integration tests (starts ephemeral K8s API server):

```bash
make test-integration
```

### Verbose Output

For detailed test output:

```bash
make test-verbose
```

### With Coverage Report

Generate HTML coverage report:

```bash
make test-coverage
```

This creates `coverage.html` with visual coverage data.

## Writing Tests

### Integration Tests

Integration tests validate the validation engine against a real Kubernetes API server. Use the `helpers.SetupEnvTest()` function to create a test environment:

```go
// +build integration

package integration

import (
    "testing"
    "github.com/kubeasy-dev/kubeasy-cli/test/helpers"
)

func TestMyValidation(t *testing.T) {
    // Setup test environment with K8s API server
    env := helpers.SetupEnvTest(t)

    // Create resources
    pod := env.CreatePod(&corev1.Pod{...})
    env.SetPodReady(pod.Name)

    // Run validation
    executor := validation.NewExecutor(env.Clientset, env.DynamicClient, env.Config, env.Namespace)
    result := executor.Execute(ctx, validation)

    // Assert
    assert.True(t, result.Passed)
}
```

### Test Helpers

The `helpers` package provides utilities for integration tests:

- `SetupEnvTest(t)` - Creates isolated K8s test environment
- `env.CreatePod(pod)` - Creates a pod
- `env.SetPodReady(name)` - Marks pod as Ready
- `env.UpdatePodStatus(pod)` - Updates pod status
- `env.CreateEvent(event)` - Creates K8s event
- `env.WaitForPod(name, timeout)` - Waits for pod to exist

### Build Tags

Integration tests use build tags to separate them from unit tests:

```go
// +build integration

package integration
```

This allows running them selectively with `-tags=integration`.

## CI/CD

Tests run automatically in GitHub Actions on:
- Push to `main` branch
- Pull requests to `main`
- Push to `feat/**` and `fix/**` branches

See `.github/workflows/test.yml` for the full CI configuration.

### CI Test Command

The CI uses a specialized target that combines coverage from unit and integration tests:

```bash
make ci-test
```

## Test Coverage

Current coverage targets:
- **Overall**: ≥60%
- **pkg/validation**: ≥70%

View coverage report:

```bash
make test-coverage
open coverage.html
```

## Troubleshooting

### "setup-envtest: command not found"

Install the envtest setup tool:

```bash
go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
export PATH=$PATH:$(go env GOPATH)/bin
```

### "unable to start control plane"

Ensure envtest binaries are installed:

```bash
make setup-envtest
```

### Tests Timeout

Increase timeout for integration tests:

```bash
go test -tags=integration ./test/integration -timeout 15m
```

### Clean Test Cache

Clear Go test cache:

```bash
go clean -testcache
```

## Performance

Integration test performance on M1 Mac:
- Single test: ~4-5 seconds
- Full integration suite: ~2-3 minutes
- EnvTest startup: ~3-4 seconds

## Future Work

- [ ] Log validation integration tests
- [ ] Connectivity validation integration tests
- [ ] Unit tests for pkg/api, pkg/kube, pkg/argocd
- [ ] E2E tests with real Kind cluster
- [ ] Performance benchmarks
