# Phase 7: Probe Pod + Internal Connectivity - Context

**Gathered:** 2026-03-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Extend the connectivity validation type to auto-manage a curl probe pod when no `sourcePod` is specified, enable cross-namespace source pod lookups, assert that a connection is blocked (`expectedStatusCode: 0`), and remove the wget fallback. External HTTP and TLS are out of scope (Phases 8–9).

</domain>

<decisions>
## Implementation Decisions

### Probe pod concurrency model
- One probe pod per `ConnectivitySpec` execution (not per validation key, not one global pod per submit run)
- Created at the start of `executeConnectivity` when `SourcePod` has no name/labelSelector; deleted immediately after that spec's checks complete
- Lifecycle functions (`CreateProbePod`, `DeleteProbePod`) live in `deployer/` — executor stays cluster-read-only for other operations
- If `kubeasy-probe` already exists in the target namespace (stale from a prior crash): delete and recreate unconditionally — no reuse

### Ctrl-C cleanup (PROBE-03)
- Probe pod deletion uses `context.Background()` with a fixed timeout (e.g. 10 s) — independent from the cancelled validation context
- Guarantees cleanup even on interrupt, no signal handler changes needed

### Blocked connection semantics (CONN-01)
- `expectedStatusCode: 0` means: any curl failure passes (connection refused, timeout, TCP reset, etc.)
- Detection in `checkConnectivity`: if `target.ExpectedStatusCode == 0` and the `exec.StreamWithContext` call returns an error, that is a PASS
- `buildCurlCommand` is unchanged — same flags, same no-shell contract
- Timeout for status-0 checks is user-controlled via `TimeoutSeconds` in spec (challenge authors set a short value like 3–5 s for NetworkPolicy tests)
- All other HTTP status codes (200, 403, 503, etc.) already work correctly via curl `-w "%{http_code}"` output

### Spec schema changes
- Add `Namespace` field to the existing `SourcePod` struct (not a top-level field on `ConnectivitySpec`)
- One `Namespace` field covers both use cases:
  - Auto-probe pod: no name/labelSelector → probe pod is created in `SourcePod.Namespace` (defaults to challenge namespace if empty)
  - Cross-namespace existing pod (CONN-02): name or labelSelector present → executor queries `SourcePod.Namespace` instead of `e.namespace`
- `SourcePod.Namespace` always wins over executor's default namespace when set

### Probe pod image and identity
- Image: `curlimages/curl` — version pinned in `internal/deployer/const.go`, Renovate-managed
- Pod name: `kubeasy-probe` (fixed, not random)
- Labels: `{app: kubeasy-probe, managed-by: kubeasy}` — enables challenge authors to write NetworkPolicy rules targeting the probe by label
- ServiceAccount: namespace default — no dedicated SA created (pod identity for NetworkPolicy = labels, not SA)

### wget fallback removal (PROBE-04)
- Remove the `TODO(sec)` wget fallback block in `checkConnectivity` entirely
- If curl exec fails (not a status-0 check), return the curl error directly — no fallback attempt
- Probe pod image guarantees curl is available — the fallback was only needed for unknown user pods

### Claude's Discretion
- Exact probe pod spec (resource requests, restart policy, image pull policy)
- WaitForProbePodReady implementation details (poll interval, max wait duration)
- Error messages for probe pod creation failures
- Whether `CreateProbePod` returns the pod object or just an error

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `buildCurlCommand(url, timeoutSeconds)` in `executor.go` — already returns arg slice, no shell, reuse unchanged for probe pod exec
- `kube.CreateNamespace(ctx, clientset, ns)` — may be needed if `SourcePod.Namespace` targets a namespace the probe pod needs
- `kube.WaitForDeploymentsReady` — pattern to replicate for `WaitForProbePodReady` (poll until Running)
- `deployer/infrastructure.go` — home for `CreateProbePod` / `DeleteProbePod` functions (consistent with existing deployer pattern)
- `internal/deployer/const.go` — add `ProbePodImage` and `ProbePodImageVersion` constants (Renovate-managed)

### Established Patterns
- `IsInfrastructureReadyWithClient(ctx, clientset)` — testable pattern with injected client; replicate for probe pod functions
- Version constants with Renovate comment annotations in `const.go`
- `context.Background()` with explicit timeout for cleanup operations (established in Phase 6 cert-manager webhook polling)
- `buildCurlCommand` no-shell contract — locked by v1.0 SEC-01; probe pod exec must follow same pattern

### Integration Points
- `internal/validation/executor.go` → `executeConnectivity()` — entry point for probe pod create/delete calls
- `internal/validation/types.go` → `SourcePod` struct — add `Namespace string` field here
- `internal/deployer/` — new file `probe.go` or additions to `infrastructure.go` for `CreateProbePod`/`DeleteProbePod`
- `internal/constants/const.go` — may need probe pod name constant (`KubeasyProbePodName = "kubeasy-probe"`)

</code_context>

<specifics>
## Specific Ideas

- Probe pod labels `{app: kubeasy-probe, managed-by: kubeasy}` are intentionally stable so challenge authors can write NetworkPolicy rules targeting them — this is a design feature, not just an implementation detail
- The `SourcePod.Namespace` unified model (one field for both auto-probe placement and cross-namespace existing pod lookup) keeps the spec flat and avoids two separate namespace concepts

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 07-probe-pod-internal-connectivity*
*Context gathered: 2026-03-11*
