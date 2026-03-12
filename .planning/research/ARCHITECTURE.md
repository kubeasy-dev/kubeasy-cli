# Architecture Research

**Domain:** Go CLI — Kubernetes connectivity validation extension
**Researched:** 2026-03-11
**Confidence:** HIGH (all findings grounded in live source code)

---

## Standard Architecture

### System Overview (current + v2.7.0 additions)

```
┌─────────────────────────────────────────────────────────────────────┐
│  cmd/ (Cobra commands)                                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐            │
│  │ setup.go │  │ start.go │  │submit.go │  │ reset.go │            │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘            │
└───────┼─────────────┼─────────────┼──────────────┼──────────────────┘
        │             │             │              │
┌───────┴─────────────┴─────────────┴──────────────┴──────────────────┐
│  internal/deployer/                                                   │
│  ┌─────────────────────┐  ┌────────────────┐  ┌──────────────────┐  │
│  │  infrastructure.go  │  │  challenge.go  │  │   cleanup.go     │  │
│  │  SetupInfrastructure│  │  DeployChallenge│  │ CleanupChallenge │  │
│  │  [MODIFIED v2.7.0]  │  └────────────────┘  └──────────────────┘  │
│  └─────────────────────┘                                              │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │  NEW: probe.go  (ProbePodManager — auto-deploy/cleanup)      │    │
│  └──────────────────────────────────────────────────────────────┘    │
├───────────────────────────────────────────────────────────────────────┤
│  internal/validation/                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │
│  │  executor.go │  │  loader.go   │  │   types.go   │               │
│  │  [MODIFIED]  │  │ [UNCHANGED]  │  │  [MODIFIED]  │               │
│  └──────────────┘  └──────────────┘  └──────────────┘               │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │  NEW: external.go  (executeExternal — net/http from CLI)     │    │
│  └──────────────────────────────────────────────────────────────┘    │
├───────────────────────────────────────────────────────────────────────┤
│  internal/kube/  internal/api/  internal/constants/  internal/logger/ │
│  [UNCHANGED or minor additions]                                        │
└───────────────────────────────────────────────────────────────────────┘
        │                           │
        ▼                           ▼
┌────────────────┐       ┌──────────────────────────────┐
│  Kind cluster  │       │  External (host)              │
│  kind-kubeasy  │       │  cloud-provider-kind binary   │
│                │       │  (NOT managed by CLI)         │
└────────────────┘       └──────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Modified in v2.7.0 |
|-----------|---------------|---------------------|
| `cmd/setup.go` | User-facing setup command, orchestrates deployer calls | YES — new infra steps |
| `internal/deployer/infrastructure.go` | Installs Kyverno, local-path-provisioner, and new infra | YES — adds nginx-ingress, Gateway API, cert-manager |
| `internal/deployer/probe.go` (NEW) | Lifecycle of probe pod: create, wait ready, delete after validation | NEW |
| `internal/validation/types.go` | Spec type definitions for all validation types | YES — ConnectivitySpec extended |
| `internal/validation/executor.go` | Routes to type-specific executors; executeConnectivity extended | YES — probe pod injection, mode dispatch |
| `internal/validation/external.go` (NEW) | External HTTP checks from CLI host via net/http | NEW |
| `internal/validation/loader.go` | Parses challenge.yaml specs; adds new field validation | YES — new spec fields |
| `internal/deployer/const.go` | Version pins for Renovate | YES — new version vars |

---

## Recommended Project Structure (delta only)

```
internal/
├── deployer/
│   ├── const.go               # ADD: NginxIngressVersion, GatewayAPICRDsVersion,
│   │                          #      CertManagerVersion, GatewayControllerVersion
│   ├── infrastructure.go      # MODIFY: extend SetupInfrastructure(),
│   │                          #         IsInfrastructureReady()
│   └── probe.go               # NEW: ProbePodManager
│
└── validation/
    ├── types.go               # MODIFY: extend ConnectivitySpec, ConnectivityCheck,
    │                          #         add ExternalSpec, TLSSpec (or inline fields)
    ├── executor.go            # MODIFY: extend executeConnectivity() for probe
    │                          #         dispatch; add executeExternal()
    ├── loader.go              # MODIFY: parseSpec() for new fields + validation
    └── external.go            # NEW: doExternalCheck(ctx, ExternalCheck) Result
```

No new top-level packages. All additions fit the existing `deployer` and `validation` boundaries.

---

## Architectural Patterns

### Pattern 1: "Extend, don't replace" for ConnectivitySpec

**What:** Add a `Mode` discriminant field (`internal` | `external`) to `ConnectivitySpec`. The existing `executeConnectivity()` dispatches on mode. Internal mode uses the current pod-exec path. External mode delegates to `executeExternal()` in a new file.

**When to use:** Whenever a validation type gains a new execution strategy. Keeps the `Execute()` switch unchanged — only `executeConnectivity()` grows a branch.

**Trade-offs:** Slightly more surface in `ConnectivitySpec`, but avoids a new top-level `ValidationType` that would require challenge.yaml format changes.

**Key decision:** Do NOT introduce a separate `TypeExternal` or `TypeTLS` ValidationType. That would require a backend schema change and break all existing challenge.yaml parsing. Extend `connectivity` instead.

```go
// In types.go
type ConnectivitySpec struct {
    Mode      ConnectivityMode   `yaml:"mode,omitempty"`   // "internal" (default) | "external"
    SourcePod SourcePod          `yaml:"sourcePod,omitempty"` // optional when Mode==external or probe auto-deployed
    Targets   []ConnectivityCheck `yaml:"targets"`
}

type ConnectivityMode string
const (
    ModeInternal ConnectivityMode = "internal"
    ModeExternal ConnectivityMode = "external"
)
```

### Pattern 2: Probe pod lifecycle in deployer/probe.go

**What:** `ProbePodManager` is a thin struct in `internal/deployer/` that creates a well-known probe pod (curlimages/curl or similar), waits for it to be Ready, and deletes it after the validation run. It is NOT part of the `validation` package — it is called by `cmd/submit.go` before `ExecuteAll()` and deferred for cleanup.

**When to use:** Whenever the challenge spec has `mode: internal` with no `sourcePod` (or with `sourcePod.autoDeploy: true`).

**Why deployer, not validation:** The validation package should remain cluster-read-only during execution. Deploying resources is deployer's job. The executor receives a pod reference it can use, not instructions to create pods. This mirrors the existing boundary — deployer deploys, executor reads.

**Trade-offs:** `cmd/submit.go` needs one new conditional block before/after validation. The probe pod's name/namespace is threaded into the spec at call time, not baked into `challenge.yaml`.

```go
// internal/deployer/probe.go
type ProbePodManager struct {
    clientset kubernetes.Interface
    namespace string
    podName   string
}

func (m *ProbePodManager) Deploy(ctx context.Context) error { ... }
func (m *ProbePodManager) Cleanup(ctx context.Context) error { ... }
func (m *ProbePodManager) PodName() string { return m.podName }
```

### Pattern 3: External HTTP check as a separate file within validation/

**What:** `internal/validation/external.go` contains `executeExternal(ctx, spec ConnectivitySpec) (bool, string, error)`. It uses `net/http` to make requests from the CLI host, resolves the IP/port from an Ingress or Gateway resource if needed, and validates TLS if configured.

**When to use:** Called by `executeConnectivity()` when `spec.Mode == ModeExternal`.

**Why a separate file:** External checks share zero code with pod-exec checks. Keeping them in a separate file makes the boundary clear, test surface smaller, and avoids growing `executor.go` (already 783 lines).

**Trade-offs:** One more file. Caller always goes through `executor.go` so the public API is unchanged.

### Pattern 4: Infrastructure setup remains a sequential function

**What:** `SetupInfrastructure()` in `infrastructure.go` is extended with new sequential steps: install Gateway API CRDs, install nginx-ingress (or NGINX Gateway Fabric), install cert-manager, check readiness. Same fetch-manifest + apply pattern used for Kyverno.

**When to use:** v2.7.0 adds 3-4 new steps to this function. No architectural change needed.

**Why not a loop/table of components:** The current pattern is readable and each component has different readiness checks (namespaces, deployment names, CRD presence). A generic loop would require a complex descriptor struct that adds complexity without benefit at this scale.

**Constraint — REST mapper limitation:** The existing comment in `SetupInfrastructure()` notes the mapper is a point-in-time snapshot. Gateway API CRDs must be installed first and the mapper refreshed before applying Gateway/GatewayClass objects. Use a fresh `GetAPIGroupResources` call after CRD application, or apply CRD-instance objects in a second pass with a new mapper.

---

## Data Flow

### Submit Flow (current)

```
cmd/submit.go
    → validation.LoadForChallenge(slug)
    → kube.GetKubernetesClient() / GetDynamicClient()
    → validation.NewExecutor(clientset, dynamicClient, restConfig, namespace)
    → executor.ExecuteAll(ctx, validations)
    → api.SendSubmit(slug, results)
```

### Submit Flow (v2.7.0 — internal mode with probe)

```
cmd/submit.go
    → validation.LoadForChallenge(slug)   [unchanged]
    → needsProbe := hasAutoProbe(validations)
    → if needsProbe:
          deployer.NewProbePodManager(clientset, namespace).Deploy(ctx)
          defer manager.Cleanup(ctx)
          inject probePodName into relevant ConnectivitySpec.SourcePod.Name
    → validation.NewExecutor(...)          [unchanged signature]
    → executor.ExecuteAll(ctx, validations) [modified dispatch inside]
    → api.SendSubmit(slug, results)        [unchanged]
```

### Submit Flow (v2.7.0 — external mode)

```
cmd/submit.go
    → validation.LoadForChallenge(slug)   [unchanged]
    → validation.NewExecutor(...)          [unchanged]
    → executor.ExecuteAll → executeConnectivity → executeExternal
          → resolve IP/port from Ingress/Gateway (dynamic client)
          → net/http.Do(req) from CLI host
          → TLS validation via crypto/tls
    → api.SendSubmit(slug, results)        [unchanged]
```

### Setup Flow (v2.7.0)

```
cmd/setup.go
    → deployer.IsInfrastructureReady()    [extended to check new components]
    → deployer.SetupInfrastructure()      [extended]
        → install Kyverno + local-path-provisioner  [existing]
        → install Gateway API CRDs                   [NEW - INFRA-02]
        → refresh REST mapper
        → install nginx-ingress or NGINX GW Fabric   [NEW - INFRA-01/03]
        → install cert-manager                       [NEW - INFRA-04]
        → WaitForDeploymentsReady (all new components) [NEW - INFRA-06]
    → log: "cloud-provider-kind must be running separately" [NEW - INFRA-05]
```

---

## Integration Points

### cloud-provider-kind (INFRA-05) — External Binary, Not Managed by CLI

**Architecture decision:** cloud-provider-kind is a host-side binary that monitors Docker and assigns IPs to LoadBalancer services. It runs as a persistent daemon in a separate terminal. The CLI cannot start it as a subprocess (it would block or require backgrounding with no lifecycle management).

**Recommended integration:** The CLI does NOT install or start cloud-provider-kind. `SetupInfrastructure()` checks whether any LoadBalancer service already has an external IP (as a proxy for whether cloud-provider-kind is running) and prints a warning if not: "For Ingress LoadBalancer tests, ensure cloud-provider-kind is running: `cloud-provider-kind`". This satisfies INFRA-05 ("setup integrates or documents it") without the complexity of managing an external daemon.

**Confidence:** HIGH — confirmed in cloud-provider-kind README: "run the cloud-provider-kind in a terminal and keep it running." No in-process or in-cluster deployment path exists.

### nginx-ingress vs. NGINX Gateway Fabric

**Recommendation:** Install NGINX Gateway Fabric (NGF), not the legacy ingress-nginx controller. NGF implements both the Ingress resource (for backward compatibility) and the Gateway API. This means one controller handles INFRA-01 and INFRA-03 in a single install step.

**Install path:** Manifest-based, from `https://raw.githubusercontent.com/nginx/nginx-gateway-fabric/<version>/deploy/crds.yaml` then the deployment manifest. Fits cleanly into the existing fetch-manifest + apply pattern. No Helm required.

**Confidence:** MEDIUM — NGF v2.4.x is current as of 2026-03. ingress-nginx EOL is announced for early 2026, making NGF the forward-looking choice.

### cert-manager

**Install path:** Official manifest at `https://github.com/cert-manager/cert-manager/releases/download/<version>/cert-manager.yaml`. Uses the same fetch-manifest + apply pattern. Readiness check: `cert-manager` namespace, deployments `cert-manager`, `cert-manager-cainjector`, `cert-manager-webhook`.

**REST mapper note:** cert-manager's CRDs (Issuer, Certificate, etc.) are registered by the install manifest. Any challenge that applies cert-manager CRD instances must use a fresh mapper. Infrastructure setup only installs the controller — challenges use the CRDs. No second-pass mapper issue for `SetupInfrastructure()`.

**Confidence:** HIGH — official release manifest URL pattern is stable across versions.

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| `cmd/submit.go` → `deployer/probe.go` | Direct function call | Probe lifecycle is a deployer concern, not a validation concern |
| `cmd/submit.go` → `validation/executor.go` | Existing `NewExecutor` + `ExecuteAll` | Signature unchanged — probe pod name injected into spec before call |
| `executor.go` → `external.go` | Package-private function call | `executeExternal` is unexported, called only by `executeConnectivity` |
| `deployer/infrastructure.go` → `kube/manifest.go` | `kube.FetchManifest` + `kube.ApplyManifest` | Existing pattern; new URLs must be added to `fetchManifestAllowedPrefixes` |
| `validation/executor.go` → `kube` (indirect) | Via `dynamic.Interface` in `Executor` | External resolver reads Ingress/Gateway via dynamic client (already present) |

---

## Scaling Considerations

Not applicable — this is a single-user local CLI. All operations run sequentially or with bounded goroutine pools (existing `ExecuteAll` pattern). No scale dimension is relevant.

---

## Anti-Patterns

### Anti-Pattern 1: Probe pod deployment inside the validation executor

**What people do:** Have `executeConnectivity()` call Kubernetes API to create a pod when no source pod is specified, then delete it after the check.

**Why it's wrong:** The executor is currently a pure reader — it observes cluster state. Making it write resources violates this boundary, makes it significantly harder to test (need fake creation + deletion), and means cleanup code lives in an unexpected place. If validation panics or context cancels, cleanup may not run.

**Do this instead:** ProbePodManager in `deployer/` with deferred Cleanup in `cmd/submit.go`. Executor receives a fully-resolved source pod name, stays read-only.

### Anti-Pattern 2: New ValidationType for external and TLS checks

**What people do:** Add `TypeExternal` and `TypeTLS` constants, add cases to the `Execute()` switch, add new spec types, update loader `parseSpec()`, and update backend submission format.

**Why it's wrong:** The backend verifies that `objectiveKey` values match registered objectives. A challenge.yaml with `type: external` would fail on all deployed backend instances until they are updated. challenge.yaml is stored in the challenges repo — format changes require coordinated releases across three repos.

**Do this instead:** Extend `ConnectivitySpec` with a `Mode` discriminant and inline TLS options on `ConnectivityCheck`. Existing `type: connectivity` in challenge.yaml continues to work; new fields are optional with zero-value defaults.

### Anti-Pattern 3: Hardcoding cloud-provider-kind as a required dependency

**What people do:** Make `SetupInfrastructure()` fail if `cloud-provider-kind` is not detected in PATH, or attempt to start it via `os.Exec`.

**Why it's wrong:** cloud-provider-kind is a host daemon, not a cluster component. On macOS and WSL2 it requires `sudo`. Requiring it at setup time forces users who only want NetworkPolicy challenges (internal mode) to install an extra binary. It cannot be managed as a subprocess without blocking or orphaning.

**Do this instead:** Print a conditional warning in setup output and in the challenge instructions. Connectivity validations that resolve via LoadBalancer IP fail with a clear error ("no external IP assigned — is cloud-provider-kind running?"), not a setup gate.

### Anti-Pattern 4: Expanding fetchManifestAllowedPrefixes blindly

**What people do:** Add `https://` as a catch-all allowed prefix, or add every new vendor domain as needed.

**Why it's wrong:** `fetchManifestAllowedPrefixes` is a security control (SEC-02). Widening it reduces the protection it provides against supply-chain manifest injection.

**Do this instead:** Add only the specific prefixes required: `https://github.com/cert-manager/`, `https://raw.githubusercontent.com/nginx/`, `https://github.com/nginx/`. Each new component needs exactly one prefix entry, reviewed at PR time.

---

## Suggested Build Order

Dependencies drive this order. Each phase leaves the codebase in a working state.

| Step | Scope | Rationale |
|------|-------|-----------|
| 1 | Extend `ConnectivitySpec` + `ConnectivityCheck` in `types.go` — add `Mode`, `Namespace`, `HostHeader`, TLS fields | Types must exist before anything else can compile |
| 2 | Update `loader.go` `parseSpec()` — accept new optional fields, keep zero-value defaults | Loader feeds executor; must handle new fields before executor uses them |
| 3 | Remove wget fallback (PROBE-04) from `executor.go` — curl-only | Clean up the known `TODO(sec)` before adding more code around it |
| 4 | Implement `deployer/probe.go` (ProbePodManager) | No deps on validator changes; can be implemented and tested independently |
| 5 | Extend `executeConnectivity()` in `executor.go` — probe injection (PROBE-01/02/03), cross-namespace source (CONN-02), mode dispatch | Depends on steps 1, 3, 4 |
| 6 | Implement `validation/external.go` — `executeExternal()` with net/http, Host header, IP/port resolution from Ingress/Gateway (EXT-01/02/03/04) | Depends on step 5 (called from executeConnectivity) |
| 7 | Add TLS validation within `external.go` — cert expiry, SAN check, insecureSkipVerify (TLS-01/02/03) | Depends on step 6; crypto/tls is stdlib, no new deps |
| 8 | Extend `deployer/infrastructure.go` — Gateway API CRDs, NGINX Gateway Fabric, cert-manager (INFRA-01/02/03/04) | Independent of validation changes; can proceed in parallel with steps 5-7 |
| 9 | Extend `deployer/infrastructure.go` `IsInfrastructureReady()` — check new components (INFRA-06) | Depends on step 8 (component names known) |
| 10 | Update `cmd/setup.go` — wire new steps, cloud-provider-kind advisory message (INFRA-05) | Depends on steps 8, 9 |
| 11 | Update `cmd/submit.go` — probe pod lifecycle wiring | Depends on steps 4, 5 |
| 12 | Update `deployer/const.go` — add version vars for new components with Renovate annotations | Can be done alongside step 8 |

Steps 4, 8, and 12 are independent and can be worked in parallel. Steps 1-3 are prerequisites for all validation work.

---

## Sources

- Live source: `/internal/validation/executor.go`, `/internal/validation/types.go`, `/internal/deployer/infrastructure.go`, `/internal/deployer/const.go`, `/internal/kube/manifest.go`, `/cmd/setup.go`
- [kubernetes-sigs/cloud-provider-kind](https://github.com/kubernetes-sigs/cloud-provider-kind) — external binary architecture confirmed
- [NGINX Gateway Fabric manifest install](https://docs.nginx.com/nginx-gateway-fabric/install/manifests/open-source/) — MEDIUM confidence, v2.4.x current
- [cert-manager + Gateway API transition announcement](https://cert-manager.io/announcements/2025/11/26/ingress-nginx-eol-and-gateway-api/) — ingress-nginx EOL context

---
*Architecture research for: kubeasy-cli v2.7.0 connectivity extension*
*Researched: 2026-03-11*
