# Phase 6: Infrastructure Foundation - Context

**Gathered:** 2026-03-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Extend `kubeasy setup` to install three new components (nginx-ingress, Gateway API CRDs + GatewayClass, cert-manager) and manage cloud-provider-kind as an auto-installed background process. Each component reports its status individually. Existing components (Kyverno, local-path-provisioner) are retrofitted into the same per-component model. Challenges and validation are out of scope for this phase.

</domain>

<decisions>
## Implementation Decisions

### Status output format
- Sequential per-component lines, not a summary table: each component prints its own ✓/✗/⚠ line as it completes
- Status labels: `ready`, `not-ready`, `missing` (matches INFRA-07 spec language)
- Retrofit existing components (Kyverno, local-path-provisioner) into the same model — all 5 components use identical status reporting
- Keep the final `"Kubeasy environment is ready!"` success footer (current behavior)

### Idempotency
- Per-component skip: each component is checked independently — already ready → show `✓ ready`, skip install; missing/unhealthy → install
- Continue-on-failure: if one component install fails, mark it `not-ready` and proceed with remaining components. User can re-run setup to retry.
- Refactor `SetupInfrastructure()` into per-component functions (e.g. `installNginxIngress()`, `installCertManager()`) — enables independent skip logic and cleaner testing

### Existing cluster + port mappings (INFRA-06)
- Store the Kind cluster config at `~/.kubeasy/kind-config.yaml` — written during cluster creation, read during setup to detect if port mappings are present
- If cluster exists and kind-config.yaml is missing or lacks extraPortMappings: ask for confirmation, then auto-recreate cluster with correct config (ports 8080/8443)
- No silent recreation — always confirm with user before destructive operation

### cloud-provider-kind (INFRA-03, INFRA-05)
- **Architecture change from original design**: do NOT require user to install/run cloud-provider-kind separately
- Download binary automatically from GitHub Releases to `~/.kubeasy/bin/cloud-provider-kind` — version pinned in const.go (Renovate-managed like KyvernoVersion)
- Start as detached background OS process after download — keeps running after CLI exits to continuously serve LoadBalancer IP assignments
- If already running (process check): skip download/start, show `✓ ready`
- This replaces the "detection + advisory" approach from the original design

### Gateway API CRDs (INFRA-02, INFRA-03)
- Two-pass apply: apply CRDs first, rebuild REST mapper, then apply GatewayClass resource
- CRDs pinned to v1.2.1 Standard channel (not v1.5.0 — requires server-side apply which we don't support)
- GatewayClass backed by cloud-provider-kind (bundled in cloud-provider-kind binary, not a separate install)

### cert-manager (INFRA-04)
- Two-pass apply: CRDs first, then controller deployment
- Readiness check: poll cert-manager-webhook Endpoints object (not just ReadyReplicas) — needs 15–30s post-Ready polling

### nginx-ingress (INFRA-01)
- Use Kind-specific manifest from ingress-nginx v1.15.0 (has correct hostNetwork + tolerations for Kind)
- extraPortMappings on 8080/8443 in Kind cluster config enable host-level access

### Claude's Discretion
- Exact process detection approach for cloud-provider-kind "is running" check
- REST mapper rebuild implementation details for two-pass Gateway API apply
- Error message wording for cluster recreation prompt
- Internal function signatures for per-component installers

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `kube.FetchManifest(url)` — fetches manifest bytes from URL, already validates against trusted prefixes
- `kube.ApplyManifest(ctx, manifest, ns, mapper, dynamicClient)` — applies YAML manifest to cluster
- `kube.WaitForDeploymentsReady(ctx, clientset, ns, names)` — waits for deployment readiness
- `kube.CreateNamespace(ctx, clientset, ns)` — idempotent namespace creation
- `deployer.KyvernoVersion` pattern — Renovate-managed version var in const.go
- `ui.TimedSpinner()`, `ui.Success()`, `ui.Error()`, `ui.Warning()` — existing output primitives
- `cluster.NewProvider()` from `sigs.k8s.io/kind/pkg/cluster` — already used for cluster creation

### Established Patterns
- Version constants in `internal/deployer/const.go` with Renovate comment annotations
- `IsInfrastructureReadyWithClient(ctx, clientset)` — testable version with injected client (pattern to replicate per component)
- Manifest URLs constructed via functions (e.g. `kyvernoInstallURL()`) rather than inline constants
- `cmd/setup.go` orchestrates deployer calls and handles UI output

### Integration Points
- `cmd/setup.go` — main entry point; cluster creation and infra setup calls go here
- `internal/deployer/infrastructure.go` — will be refactored into per-component functions
- `internal/deployer/const.go` — new version vars for nginx-ingress, Gateway API CRDs, cert-manager, cloud-provider-kind
- `~/.kubeasy/kind-config.yaml` — new config file path (needs a constants entry for the path)

</code_context>

<specifics>
## Specific Ideas

- User wants cloud-provider-kind managed entirely by the CLI — no separate user action required. Same pattern as how Kind is managed (library/binary, not a user-installed tool).
- The download + detach model for cloud-provider-kind mirrors how some CLI tools manage companion daemons (e.g., `colima start` behavior)

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 06-infrastructure-foundation*
*Context gathered: 2026-03-11*
