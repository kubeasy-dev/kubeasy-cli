# Phase 7: Probe Pod + Internal Connectivity - Research

**Researched:** 2026-03-11
**Domain:** Go / Kubernetes client-go / pod lifecycle management / exec remotecommand
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Probe pod concurrency model**
- One probe pod per `ConnectivitySpec` execution (not per validation key, not one global pod per submit run)
- Created at the start of `executeConnectivity` when `SourcePod` has no name/labelSelector; deleted immediately after that spec's checks complete
- Lifecycle functions (`CreateProbePod`, `DeleteProbePod`) live in `deployer/` — executor stays cluster-read-only for other operations
- If `kubeasy-probe` already exists in the target namespace (stale from a prior crash): delete and recreate unconditionally — no reuse

**Ctrl-C cleanup (PROBE-03)**
- Probe pod deletion uses `context.Background()` with a fixed timeout (e.g. 10 s) — independent from the cancelled validation context
- Guarantees cleanup even on interrupt, no signal handler changes needed

**Blocked connection semantics (CONN-01)**
- `expectedStatusCode: 0` means: any curl failure passes (connection refused, timeout, TCP reset, etc.)
- Detection in `checkConnectivity`: if `target.ExpectedStatusCode == 0` and the `exec.StreamWithContext` call returns an error, that is a PASS
- `buildCurlCommand` is unchanged — same flags, same no-shell contract
- Timeout for status-0 checks is user-controlled via `TimeoutSeconds` in spec (challenge authors set a short value like 3–5 s for NetworkPolicy tests)
- All other HTTP status codes (200, 403, 503, etc.) already work correctly via curl `-w "%{http_code}"` output

**Spec schema changes**
- Add `Namespace` field to the existing `SourcePod` struct (not a top-level field on `ConnectivitySpec`)
- One `Namespace` field covers both use cases:
  - Auto-probe pod: no name/labelSelector → probe pod is created in `SourcePod.Namespace` (defaults to challenge namespace if empty)
  - Cross-namespace existing pod (CONN-02): name or labelSelector present → executor queries `SourcePod.Namespace` instead of `e.namespace`
- `SourcePod.Namespace` always wins over executor's default namespace when set

**Probe pod image and identity**
- Image: `curlimages/curl` — version pinned in `internal/deployer/const.go`, Renovate-managed
- Pod name: `kubeasy-probe` (fixed, not random)
- Labels: `{app: kubeasy-probe, managed-by: kubeasy}` — enables challenge authors to write NetworkPolicy rules targeting the probe by label
- ServiceAccount: namespace default — no dedicated SA created (pod identity for NetworkPolicy = labels, not SA)

**wget fallback removal (PROBE-04)**
- Remove the `TODO(sec)` wget fallback block in `checkConnectivity` entirely
- If curl exec fails (not a status-0 check), return the curl error directly — no fallback attempt
- Probe pod image guarantees curl is available — the fallback was only needed for unknown user pods

### Claude's Discretion
- Exact probe pod spec (resource requests, restart policy, image pull policy)
- WaitForProbePodReady implementation details (poll interval, max wait duration)
- Error messages for probe pod creation failures
- Whether `CreateProbePod` returns the pod object or just an error

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| PROBE-01 | User can define a connectivity validation without `sourcePod` — CLI auto-deploys `kubeasy-probe` pod with curl in the specified namespace | `executeConnectivity` default branch → `CreateProbePod` in deployer/, `WaitForProbePodReady` poll loop |
| PROBE-02 | Challenge spec can specify probe pod namespace (`probeNamespace` field, default: challenge namespace) | `SourcePod.Namespace` field in types.go; executor resolves namespace at runtime |
| PROBE-03 | Probe pod is deleted after validation via independent cleanup context (not the cancelled validation context) | `context.Background()` + fixed timeout pattern, established in Phase 6 cert-manager webhook polling |
| PROBE-04 | wget fallback `sh -c` removed from `checkConnectivity` — curl only, fix TODO(sec) | Direct deletion of the fallback block in executor.go; error propagated directly |
| CONN-01 | User can test a blocked connection (status code 0 = timeout/refused) | Guard in `checkConnectivity`: if `ExpectedStatusCode == 0` and StreamWithContext returns error → PASS |
| CONN-02 | Source pod namespace configurable in spec (`sourceNamespace` field) for cross-namespace NetworkPolicy tests | `SourcePod.Namespace` field; executor uses it instead of `e.namespace` when set |
</phase_requirements>

---

## Summary

Phase 7 extends the existing `executeConnectivity` path in `internal/validation/executor.go` with three orthogonal changes: (1) auto-managed probe pod lifecycle in `internal/deployer/`, (2) blocked-connection semantics (`expectedStatusCode: 0`), and (3) a unified `SourcePod.Namespace` field for cross-namespace source pod lookup. The wget fallback is removed as a security fix.

The codebase already supplies all structural building blocks: `wait.PollUntilContextTimeout` from `k8s.io/apimachinery/pkg/util/wait` is used throughout `kube/client.go` for readiness polling; `corev1.Pod` creation is straightforward via `clientset.CoreV1().Pods(ns).Create()`; the executor already holds `clientset kubernetes.Interface` for write operations in the probe pod namespace. The only architectural tension is that `deployer/` functions currently accept `*kubernetes.Clientset` (concrete type) in some places and `kubernetes.Interface` in others — probe pod functions must accept `kubernetes.Interface` to stay testable.

**Primary recommendation:** Add `Namespace string` to `SourcePod` in `types.go`, update `validateSourcePod` in `loader.go` to allow empty name+labelSelector (probe mode), create `internal/deployer/probe.go` with `CreateProbePod` / `DeleteProbePod` / `WaitForProbePodReady`, then wire them into `executeConnectivity` with an independent cleanup context.

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `k8s.io/client-go/kubernetes` | already in go.mod | Pod CRUD via CoreV1().Pods() | Project-standard kubernetes client |
| `k8s.io/apimachinery/pkg/util/wait` | already in go.mod | Poll-until-ready pattern | Used by `kube.WaitForDeploymentsReady` and `waitForCertManagerWebhookEndpoints` |
| `k8s.io/api/core/v1` | already in go.mod | Pod spec and status types | Standard K8s API types |
| `k8s.io/apimachinery/pkg/api/errors` | already in go.mod | IsNotFound checks on delete | Used throughout deployer/ |
| `k8s.io/client-go/tools/remotecommand` | already in go.mod | Pod exec (unchanged) | Already used in checkConnectivity |
| `context` stdlib | — | Independent cleanup context | `context.Background()` + WithTimeout |

### No New Dependencies
All required packages are already present in go.mod. No new imports at the module level.

---

## Architecture Patterns

### Recommended File Layout Changes

```
internal/
├── deployer/
│   ├── const.go          # Add ProbePodImage, ProbePodImageVersion, ProbePodName
│   └── probe.go          # New: CreateProbePod, DeleteProbePod, WaitForProbePodReady
├── validation/
│   ├── types.go          # Add Namespace field to SourcePod struct
│   ├── loader.go         # Update validateSourcePod: allow empty name+labelSelector
│   └── executor.go       # Update executeConnectivity + checkConnectivity
internal/constants/const.go  # Add KubeasyProbePodName = "kubeasy-probe"
```

### Pattern 1: Probe Pod Namespace Resolution

When `executeConnectivity` determines the source namespace:

```go
// Resolve source namespace: SourcePod.Namespace wins, else executor default
sourceNamespace := e.namespace
if spec.SourcePod.Namespace != "" {
    sourceNamespace = spec.SourcePod.Namespace
}
```

This single resolution point covers both PROBE-01/02 (auto-probe placement) and CONN-02 (existing cross-namespace pod lookup).

### Pattern 2: Auto-Probe Lifecycle in executeConnectivity

```go
// In executeConnectivity, default branch (no name, no labelSelector):
probePod, err := deployer.CreateProbePod(ctx, e.clientset, sourceNamespace)
if err != nil {
    return false, "", fmt.Errorf("failed to create probe pod: %w", err)
}

// Deferred cleanup with independent context (PROBE-03)
defer func() {
    cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    _ = deployer.DeleteProbePod(cleanupCtx, e.clientset, sourceNamespace)
}()

if err := deployer.WaitForProbePodReady(ctx, e.clientset, sourceNamespace); err != nil {
    return false, "", fmt.Errorf("probe pod not ready: %w", err)
}
sourcePod = probePod
```

The `defer` runs even when the validation context is cancelled (Ctrl-C). The `context.Background()` + `WithTimeout(10s)` pattern is already established in Phase 6 for cert-manager webhook polling.

### Pattern 3: CreateProbePod in deployer/probe.go

```go
// Source: k8s.io/api/core/v1 Pod struct
func CreateProbePod(ctx context.Context, clientset kubernetes.Interface, namespace string) (*corev1.Pod, error) {
    // Delete stale probe pod if it exists (unconditional — no reuse)
    _ = DeleteProbePod(ctx, clientset, namespace)

    pod := &corev1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name:      ProbePodName,   // "kubeasy-probe" from const.go
            Namespace: namespace,
            Labels: map[string]string{
                "app":        "kubeasy-probe",
                "managed-by": "kubeasy",
            },
        },
        Spec: corev1.PodSpec{
            RestartPolicy: corev1.RestartPolicyNever,
            Containers: []corev1.Container{
                {
                    Name:            "curl",
                    Image:           probePodImage(), // "curlimages/curl:VERSION"
                    Command:         []string{"sleep", "infinity"},
                    ImagePullPolicy: corev1.PullIfNotPresent,
                    Resources: corev1.ResourceRequirements{
                        Requests: corev1.ResourceList{
                            corev1.ResourceCPU:    resource.MustParse("10m"),
                            corev1.ResourceMemory: resource.MustParse("16Mi"),
                        },
                    },
                },
            },
        },
    }
    return clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
}
```

**Note on resource requests:** Minimal requests prevent the probe pod from disrupting challenge workloads on the memory-constrained Kind node. Claude's discretion per CONTEXT.md.

### Pattern 4: WaitForProbePodReady

Mirrors `kube.WaitForDeploymentsReady` pattern using `wait.PollUntilContextTimeout`:

```go
func WaitForProbePodReady(ctx context.Context, clientset kubernetes.Interface, namespace string) error {
    return wait.PollUntilContextTimeout(ctx, 1*time.Second, 30*time.Second, true,
        func(ctx context.Context) (bool, error) {
            pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, ProbePodName, metav1.GetOptions{})
            if err != nil {
                return false, nil // retry
            }
            return pod.Status.Phase == corev1.PodRunning, nil
        },
    )
}
```

Poll interval 1 s (vs 2 s for deployments) because probe pod startup is fast on Kind (image already cached after first use). Max wait 30 s.

### Pattern 5: Blocked Connection (CONN-01) in checkConnectivity

```go
// Before parsing stdout as status code:
if err != nil {
    if target.ExpectedStatusCode == 0 {
        return true, fmt.Sprintf("Connection to %s blocked as expected", target.URL)
    }
    return false, fmt.Sprintf("Connection to %s failed: %v", target.URL, err)
}
// ... rest of existing status code comparison
```

The existing `StreamWithContext` error return is the signal. When curl cannot connect (timeout, refused, TCP reset), the exec returns a non-nil error. This is distinct from curl succeeding but returning a non-200 status code (which flows through stdout parsing).

### Pattern 6: validateSourcePod Update in loader.go

Currently `validateSourcePod` rejects empty name+labelSelector. With probe mode this must become valid:

```go
// Before: both empty → error (probe mode now valid)
func validateSourcePod(sourcePod SourcePod) error {
    // Empty name + empty labelSelector = probe mode (valid)
    // Non-empty namespace alone is also valid (probe will be created there)
    return nil
}
```

The validation at parse time can be relaxed to accept probe mode. Runtime behavior in `executeConnectivity` handles the branch logic.

### Anti-Patterns to Avoid

- **Reusing a stale probe pod:** If `kubeasy-probe` exists from a crashed prior run, do not exec into it — delete and recreate. The old pod may be in an unknown state (image pull failure, OOMKilled).
- **Using validation context for cleanup:** When the user hits Ctrl-C, the validation context is cancelled. `defer DeleteProbePod(ctx, ...)` with the cancelled context will fail. Always use `context.Background()` + fixed timeout.
- **Blocking delete on NotFound:** `DeleteProbePod` must treat `apierrors.IsNotFound` as success (idempotent delete).
- **Shell injection via wget:** The entire wget `sh -c` fallback must be deleted, not commented out. The `TODO(sec)` comment documents the intent.
- **Namespace mis-routing for existing cross-namespace pods:** When `SourcePod.Name` or `SourcePod.LabelSelector` is set but `SourcePod.Namespace` is also set, the explicit namespace wins. The executor must query `sourceNamespace` (resolved), not `e.namespace`, for the pod lookup.
- **`*kubernetes.Clientset` vs `kubernetes.Interface`:** `CreateProbePod` / `DeleteProbePod` / `WaitForProbePodReady` must accept `kubernetes.Interface` (not the concrete `*kubernetes.Clientset`) to be testable with `fake.NewClientset()`. Only `kube.WaitForDeploymentsReady` requires the concrete type — probe pod waiting uses `wait.PollUntilContextTimeout` directly.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Pod readiness polling | Custom sleep loop | `wait.PollUntilContextTimeout` from `k8s.io/apimachinery/pkg/util/wait` | Handles context cancellation, deadline propagation, already imported |
| Pod delete with 404 check | Raw delete + error check | `apierrors.IsNotFound(err)` pattern | Already used in `kube.DeleteNamespace` and deployer/ throughout |
| Curl exec from pod | Custom HTTP client | Existing `buildCurlCommand` + `remotecommand.NewSPDYExecutor` | Already implemented, SEC-01 locked — no shell |
| Namespace creation for probe | Custom namespace logic | `kube.CreateNamespace(ctx, clientset, ns)` | Already handles AlreadyExists race, WaitForActive |

---

## Common Pitfalls

### Pitfall 1: Probe Pod Cleanup on Ctrl-C
**What goes wrong:** Using the validation context for cleanup — `DeleteProbePod(ctx, ...)` where ctx is already cancelled. The Kubernetes API call fails immediately with context error, pod is leaked.
**Why it happens:** Natural instinct to pass the existing context through.
**How to avoid:** Dedicate a `defer` with `context.Background()` + `context.WithTimeout(context.Background(), 10*time.Second)`. The 10 s ceiling prevents the cleanup from hanging indefinitely on API server issues.
**Warning signs:** Integration test where ctx is pre-cancelled still expects pod to be deleted.

### Pitfall 2: loader.go validateSourcePod Blocks Probe Mode
**What goes wrong:** `Parse()` fails with "sourcePod must specify either name or labelSelector" when a challenge YAML uses connectivity without a sourcePod (probe mode).
**Why it happens:** The current `validateSourcePod` enforces name or labelSelector. Probe mode intentionally omits both.
**How to avoid:** Update `validateSourcePod` to accept empty name+labelSelector as valid (probe mode). The runtime branching in `executeConnectivity` handles what to do.

### Pitfall 3: Status-0 Check with Non-Error Curl Exit
**What goes wrong:** Curl exits 0 (success) but connects to a different address or gets an unexpected status code. The blocked-connection check passes when it shouldn't.
**Why it happens:** Misconfigured URL that actually resolves to something. `expectedStatusCode: 0` should only pass when the exec itself errors (network unreachable, timeout).
**How to avoid:** The guard must be on `err != nil` from `StreamWithContext`, not on the stdout value. If err is nil (curl ran and returned something), proceed to normal status code comparison even if `ExpectedStatusCode == 0`. This means a status-0 spec where curl actually succeeds and returns any code will fail — which is the correct behavior (the connection was NOT blocked).

### Pitfall 4: Probe Pod Exec Namespace Mismatch
**What goes wrong:** The probe pod is created in `sourceNamespace` but `checkConnectivity` still uses `e.namespace` for the pod exec request.
**Why it happens:** `checkConnectivity` hardcodes `Namespace(e.namespace)` in the RESTClient post.
**How to avoid:** Pass the resolved `sourceNamespace` into `checkConnectivity` (or extract it from `pod.Namespace` which is already set on the `*corev1.Pod` object returned from Create). Reading `pod.Namespace` is the safest approach — it comes directly from the API response.

### Pitfall 5: Stale Probe Pod Delete Race
**What goes wrong:** `DeleteProbePod` sends the delete and returns, but the pod hasn't been removed from API server yet. `CreateProbePod` immediately re-creates the same-name pod and gets a 409 Conflict.
**Why it happens:** Kubernetes pod deletion is asynchronous — the API accepts the delete but the pod lingers in Terminating state.
**How to avoid:** After `DeleteProbePod` in the stale-pod path, wait for the pod to disappear before creating. A short `wait.PollUntilContextTimeout` checking for `IsNotFound` works. Alternatively, use `DeleteOptions` with `GracePeriodSeconds: ptr(0)` to request immediate termination, then poll for disappearance. For probe pods (RestartPolicy: Never, no finalizers), immediate deletion is reliable.

---

## Code Examples

### Probe Pod Constants (deployer/const.go addition)

```go
// ProbePodName is the fixed name of the CLI-managed curl probe pod.
// Fixed (not random) so labels are stable and challenge authors can target it in NetworkPolicy.
const ProbePodName = "kubeasy-probe"

// ProbePodImageVersion is the curlimages/curl image tag for the probe pod.
// IMPORTANT: The comment format below is required for Renovate. Do not modify.
// renovate: datasource=docker depName=curlimages/curl
var ProbePodImageVersion = "8.13.0"

func probePodImage() string {
    return fmt.Sprintf("curlimages/curl:%s", ProbePodImageVersion)
}
```

### executeConnectivity Updated Switch (executor.go)

```go
var sourcePod *corev1.Pod
switch {
case spec.SourcePod.Name != "":
    pod, err := e.clientset.CoreV1().Pods(sourceNamespace).Get(ctx, spec.SourcePod.Name, metav1.GetOptions{})
    if err != nil {
        return false, "", fmt.Errorf("failed to get source pod: %w", err)
    }
    sourcePod = pod
case len(spec.SourcePod.LabelSelector) > 0:
    // ... existing label selector logic, using sourceNamespace
default:
    // Probe mode: auto-deploy kubeasy-probe
    pod, err := deployer.CreateProbePod(ctx, e.clientset, sourceNamespace)
    if err != nil {
        return false, "", fmt.Errorf("failed to create probe pod: %w", err)
    }
    defer func() {
        cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        _ = deployer.DeleteProbePod(cleanupCtx, e.clientset, sourceNamespace)
    }()
    if err := deployer.WaitForProbePodReady(ctx, e.clientset, sourceNamespace); err != nil {
        return false, "", fmt.Errorf("probe pod failed to become ready: %w", err)
    }
    sourcePod = pod
}
```

### checkConnectivity — Status-0 Guard

```go
err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
    Stdout: &stdout,
    Stderr: &stderr,
})

if err != nil {
    if target.ExpectedStatusCode == 0 {
        return true, fmt.Sprintf("Connection to %s blocked as expected", target.URL)
    }
    return false, fmt.Sprintf("Connection to %s failed: %v", target.URL, err)
}

// Normal path: parse stdout as HTTP status code
statusCode := strings.TrimSpace(stdout.String())
// ...
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| wget fallback via `sh -c` | curl only, direct args | Phase 7 (this phase) | Eliminates shell injection vector, SEC-01 fully enforced |
| Fixed `e.namespace` for source pod | `SourcePod.Namespace`-aware resolution | Phase 7 (this phase) | Enables cross-namespace NetworkPolicy challenge scenarios |
| errNoSourcePodSpecified for empty SourcePod | Auto-deploy probe pod | Phase 7 (this phase) | Challenge authors no longer need to provide a curl-capable pod |

**Deprecated in this phase:**
- `validateSourcePod` strict check (replaced with probe-mode-aware version)
- wget fallback block in `checkConnectivity` (deleted, not commented)

---

## Open Questions

1. **`CreateProbePod` return signature — pod vs error**
   - What we know: CONTEXT.md marks this as Claude's discretion
   - What's unclear: If the function only returns `error`, the caller needs a separate `Get` to obtain the pod object for exec namespace resolution; if it returns `(*corev1.Pod, error)`, the caller gets the pod object directly from the Create response
   - Recommendation: Return `(*corev1.Pod, error)`. The `Create` API always returns the full Pod object; returning it avoids a redundant `Get` call and gives the caller the server-assigned `pod.Namespace` to pass into exec.

2. **GracePeriodSeconds for stale probe pod deletion**
   - What we know: The probe pod has `RestartPolicy: Never` and no finalizers
   - What's unclear: Whether zero grace period is needed to avoid the Terminating race
   - Recommendation: Use `GracePeriodSeconds: ptr(int64(0))` in the stale-delete `DeleteOptions`. This ensures the pod is removed immediately from the API server, eliminating the race with the subsequent Create.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify (assert/require) |
| Config file | none — standard `go test` |
| Quick run command | `go test ./internal/validation/... ./internal/deployer/... -run TestProbe -v` |
| Full suite command | `task test:unit` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| PROBE-01 | `executeConnectivity` with empty SourcePod creates probe pod and runs checks | unit | `go test ./internal/validation/... -run TestExecuteConnectivity_ProbeMode -v` | ❌ Wave 0 |
| PROBE-02 | `SourcePod.Namespace` is used as probe pod namespace | unit | `go test ./internal/validation/... -run TestExecuteConnectivity_ProbeNamespace -v` | ❌ Wave 0 |
| PROBE-03 | Probe pod deletion uses independent context (cancelled validation ctx still cleans up) | unit | `go test ./internal/deployer/... -run TestDeleteProbePod_CancelledContext -v` | ❌ Wave 0 |
| PROBE-04 | wget fallback is absent; curl error returned directly | unit | `go test ./internal/validation/... -run TestCheckConnectivity_NoCurlFallback -v` | ❌ Wave 0 |
| CONN-01 | `expectedStatusCode: 0` passes when exec returns error | unit | `go test ./internal/validation/... -run TestCheckConnectivity_BlockedConnection -v` | ❌ Wave 0 |
| CONN-02 | Source pod is fetched from `SourcePod.Namespace` not `e.namespace` | unit | `go test ./internal/validation/... -run TestExecuteConnectivity_CrossNamespace -v` | ❌ Wave 0 |
| PROBE-01 (deployer) | `CreateProbePod` creates pod with correct labels/image/RestartPolicy | unit | `go test ./internal/deployer/... -run TestCreateProbePod -v` | ❌ Wave 0 |
| PROBE-01 (deployer) | `WaitForProbePodReady` returns nil when pod is Running | unit | `go test ./internal/deployer/... -run TestWaitForProbePodReady -v` | ❌ Wave 0 |
| PROBE-03 (deployer) | `DeleteProbePod` treats NotFound as success (idempotent) | unit | `go test ./internal/deployer/... -run TestDeleteProbePod_NotFound -v` | ❌ Wave 0 |
| PROBE-01 (loader) | `Parse` accepts connectivity spec with empty SourcePod (probe mode) | unit | `go test ./internal/validation/... -run TestParse_ConnectivityProbeMode -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/validation/... ./internal/deployer/... -v -race`
- **Per wave merge:** `task test:unit`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/deployer/probe_test.go` — covers `CreateProbePod`, `DeleteProbePod`, `WaitForProbePodReady` (PROBE-01, PROBE-03)
- [ ] `internal/validation/executor_test.go` additions — covers probe mode, cross-namespace, blocked connection, no-fallback (PROBE-01, PROBE-02, PROBE-04, CONN-01, CONN-02)
- [ ] `internal/validation/loader_test.go` additions — covers `validateSourcePod` relaxation (PROBE-01 parse-time)

---

## Sources

### Primary (HIGH confidence)
- `internal/validation/executor.go` — full `executeConnectivity` / `checkConnectivity` / `buildCurlCommand` implementations read directly
- `internal/validation/types.go` — full `SourcePod`, `ConnectivitySpec`, `ConnectivityCheck` struct definitions
- `internal/validation/loader.go` — full `validateSourcePod` and `parseSpec` for TypeConnectivity
- `internal/deployer/infrastructure.go` — `WaitForDeploymentsReady` caller pattern, `waitForCertManagerWebhookEndpoints` using `context.Background()` + timeout for cleanup
- `internal/deployer/const.go` — Renovate annotation format for new image version constant
- `internal/kube/client.go` — `wait.PollUntilContextTimeout` usage pattern in `WaitForDeploymentsReady`
- `.planning/phases/07-probe-pod-internal-connectivity/07-CONTEXT.md` — all locked decisions

### Secondary (MEDIUM confidence)
- `internal/validation/executor_test.go` — established test patterns: `fake.NewClientset()`, `fake.NewSimpleDynamicClient()`, `assert`/`require`
- `internal/deployer/infrastructure_test.go` — `makeDeployment`, `makeNamespace` helpers; deployer unit test structure

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages already in go.mod, no new dependencies
- Architecture: HIGH — all patterns directly sourced from existing codebase; no speculation
- Pitfalls: HIGH — stale-pod race, context cancellation cleanup, and validateSourcePod breakage are concrete failure modes grounded in the actual code paths

**Research date:** 2026-03-11
**Valid until:** 2026-06-11 (stable Go k8s client-go API surface)
