# Issue #90: ArgoCD sync fails when namespace is not yet ready

**Issue Link**: https://github.com/kubeasy-dev/kubeasy-cli/issues/90

## Problem Analysis

The issue is a **race condition** between namespace creation and ArgoCD Application deployment. When the CLI runs `kubeasy challenge start <slug>`:

1. CLI creates the namespace via `kube.CreateNamespace()`
2. CLI creates the ArgoCD Application immediately after
3. ArgoCD attempts to sync but the namespace may not be fully `Active` yet
4. Sync fails because the target namespace isn't ready

### Current Code Flow (`cmd/start.go`)

```go
// Step 1: Create namespace
err = ui.WaitMessage("Creating namespace", func() error {
    return kube.CreateNamespace(ctx, staticClient, challengeSlug)
})

// Step 2: Deploy ArgoCD app (immediately after, no wait for readiness)
err = ui.WaitMessage("Deploying ArgoCD application", func() error {
    return argocd.CreateOrUpdateChallengeApplication(ctx, dynamicClient, challengeSlug)
})
```

### Current `CreateNamespace` Implementation (`pkg/kube/client.go`)

The current implementation creates the namespace but **does not wait** for it to become `Active`:

```go
func CreateNamespace(ctx context.Context, clientset kubernetes.Interface, namespace string) error {
    // ... check if exists ...
    _, err = clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
    // Returns immediately after creation - no readiness wait!
    return nil
}
```

## Solution

Following the suggestion in the issue, we'll implement a **wait for namespace readiness** approach:

1. **Create `WaitForNamespaceActive`** - A new function that polls the namespace until its status is `Active`
2. **Integrate into `CreateNamespace`** - After creating the namespace, wait for it to become `Active`

### Implementation Plan

1. Add `WaitForNamespaceActive` function to `pkg/kube/client.go`:
   - Poll interval: 500ms
   - Timeout: Use context deadline or default to 30 seconds
   - Check `ns.Status.Phase == v1.NamespaceActive`

2. Update `CreateNamespace` to call `WaitForNamespaceActive` after creation

3. Write unit tests for the new functionality

## Design Decisions

- **Poll interval**: 500ms is a reasonable balance between responsiveness and API load
- **Timeout**: Rely on context timeout from caller; if no timeout, use a sensible default (30s)
- **Logging**: Add debug logs for each poll iteration
- **Error handling**: Return clear error message on timeout

## Files to Modify

- `pkg/kube/client.go` - Add `WaitForNamespaceActive` and update `CreateNamespace`
- `pkg/kube/client_test.go` - Add tests for the new functionality

## Test Plan

1. Unit test: Verify `WaitForNamespaceActive` returns successfully when namespace is `Active`
2. Unit test: Verify `WaitForNamespaceActive` times out appropriately
3. Unit test: Verify `CreateNamespace` integration with waiting
4. Integration test: Run full `kubeasy challenge start` flow (manual)
