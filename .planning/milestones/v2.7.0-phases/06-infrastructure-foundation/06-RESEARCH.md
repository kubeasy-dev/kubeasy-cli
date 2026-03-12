# Phase 6: Infrastructure Foundation - Research

**Researched:** 2026-03-11
**Domain:** Kubernetes infrastructure setup — ingress-nginx, Gateway API CRDs, cert-manager, cloud-provider-kind, Kind cluster config
**Confidence:** HIGH (core stack), MEDIUM (cloud-provider-kind daemon details)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- Status output: sequential per-component lines — each component prints its own ✓/✗/⚠ line as it completes
- Status labels: `ready`, `not-ready`, `missing`
- Retrofit existing components (Kyverno, local-path-provisioner) into same model — all 5 components use identical status reporting
- Keep final `"Kubeasy environment is ready!"` success footer
- Idempotency: per-component skip — already ready → show `✓ ready`, skip install; missing/unhealthy → install
- Continue-on-failure: if one component install fails, mark it `not-ready` and proceed. User can re-run to retry.
- Refactor `SetupInfrastructure()` into per-component functions (e.g. `installNginxIngress()`, `installCertManager()`)
- Kind cluster config stored at `~/.kubeasy/kind-config.yaml` — written during cluster creation, read during setup to detect extraPortMappings
- If cluster exists and kind-config.yaml is missing or lacks extraPortMappings: ask for confirmation, then auto-recreate with ports 8080/8443
- No silent recreation — always confirm with user before destructive operation
- cloud-provider-kind: download binary automatically from GitHub Releases to `~/.kubeasy/bin/cloud-provider-kind` — version pinned in const.go (Renovate-managed)
- cloud-provider-kind: start as detached background OS process after download
- cloud-provider-kind: if already running (process check) → skip download/start, show `✓ ready`
- Gateway API CRDs: two-pass apply — CRDs first, rebuild REST mapper, then apply GatewayClass resource
- Gateway API CRDs pinned to v1.2.1 Standard channel (not v1.5.0)
- GatewayClass backed by cloud-provider-kind, gatewayClassName: `"cloud-provider-kind"`
- cert-manager: two-pass apply — CRDs first, then controller deployment
- cert-manager readiness: poll cert-manager-webhook Endpoints object — needs 15–30s post-Ready polling
- nginx-ingress: Kind-specific manifest from ingress-nginx v1.15.0
- extraPortMappings on 8080/8443 in Kind cluster config

### Claude's Discretion

- Exact process detection approach for cloud-provider-kind "is running" check
- REST mapper rebuild implementation details for two-pass Gateway API apply
- Error message wording for cluster recreation prompt
- Internal function signatures for per-component installers

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| INFRA-01 | nginx-ingress controller installé (ingress-nginx v1.15.0, Kind-specific manifest) | Manifest URL confirmed: `https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.15.0/deploy/static/provider/kind/deploy.yaml` |
| INFRA-02 | Gateway API v1.2.1 Standard channel CRDs installed | Install URL confirmed: `https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.1/standard-install.yaml` — standard channel does NOT require server-side apply |
| INFRA-03 | Gateway API controller via cloud-provider-kind enabled (bundled, auto-managed) | GatewayClass name: `"cloud-provider-kind"` — binary auto-downloaded from GitHub Releases, detached background process |
| INFRA-04 | cert-manager v1.19.4 installed (two-pass: CRDs then controller) | CRDs URL: `cert-manager.crds.yaml`; controller URL: `cert-manager.yaml` — both from GitHub Releases v1.19.4 |
| INFRA-05 | Clear message if cloud-provider-kind not running (now superseded by INFRA-03 auto-install per CONTEXT.md) | Process detection via `mitchellh/go-ps` or simple `exec.Command("pgrep")` |
| INFRA-06 | Kind cluster created with extraPortMappings on 8080/8443 | `cluster.CreateWithV1Alpha4Config()` or `cluster.CreateWithRawConfig()` — both available in `sigs.k8s.io/kind v0.31.0` |
| INFRA-07 | `kubeasy setup` reports status per component (ready/not-ready/missing) | `ui.Success()`, `ui.Error()`, `ui.Warning()` already exist — per-component result struct pattern |
</phase_requirements>

---

## Summary

Phase 6 extends `kubeasy setup` to install three new infrastructure components (nginx-ingress, Gateway API CRDs + GatewayClass via cloud-provider-kind, cert-manager) on top of existing Kyverno and local-path-provisioner. The primary architectural change is decomposing `SetupInfrastructure()` into per-component functions with independent idempotency checks, then retrofitting existing components into the same model.

The most novel piece is cloud-provider-kind: auto-downloaded as a binary to `~/.kubeasy/bin/` and started as a detached background process using `cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}`. This runs after CLI exit and continuously assigns LoadBalancer IPs. Process detection (is it already running?) is the one area left to Claude's discretion — simple approach: scan running processes for the binary name.

Two components require two-pass apply to avoid REST mapper errors: Gateway API (CRDs first, then GatewayClass custom resource) and cert-manager (CRDs first, then controller). Both use the established `restmapper.GetAPIGroupResources` → rebuild pattern. cert-manager webhook needs an additional 15–30 s post-deployment polling on the Endpoints object before it is truly ready.

**Primary recommendation:** Model each component as a `ComponentResult{Name, Status, Message}` struct returned from `installX(ctx, clientset, dynamicClient) ComponentResult`. The `setup.go` command collects results and prints per-line status. Failed components are logged but execution continues.

---

## Standard Stack

### Core (all already in go.mod — no new imports needed for base work)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `sigs.k8s.io/kind/pkg/cluster` | v0.31.0 | Cluster creation with config | Already used; `CreateWithV1Alpha4Config` / `CreateWithRawConfig` available |
| `sigs.k8s.io/kind/pkg/apis/config/v1alpha4` | v0.31.0 | Typed Kind cluster config struct for extraPortMappings | Part of kind library already in go.mod |
| `k8s.io/client-go/restmapper` | v0.35.0 | REST mapper rebuild after CRD install | Already used in `infrastructure.go` |
| `os/exec` + `syscall` | stdlib | Detach cloud-provider-kind as background process | Standard Go; already used in `image.go` |
| `k8s.io/apimachinery/pkg/util/wait` | v0.35.0 | Poll webhook Endpoints readiness | Already used in `kube/client.go` |

### New Version Constants (in `internal/deployer/const.go`)

| Constant | Value | Renovate datasource |
|----------|-------|---------------------|
| `NginxIngressVersion` | `"v1.15.0"` | `github-releases` depName=`kubernetes/ingress-nginx` |
| `GatewayAPICRDsVersion` | `"v1.2.1"` | `github-releases` depName=`kubernetes-sigs/gateway-api` |
| `CertManagerVersion` | `"v1.19.4"` | `github-releases` depName=`cert-manager/cert-manager` |
| `CloudProviderKindVersion` | `"v0.10.0"` | `github-releases` depName=`kubernetes-sigs/cloud-provider-kind` |

### Process Detection (Claude's Discretion — two options)

| Option | Approach | Tradeoff |
|--------|----------|----------|
| A | `exec.Command("pgrep", "-f", "cloud-provider-kind")` | Simple, Unix-only (Kind is Linux/macOS only, acceptable) |
| B | `mitchellh/go-ps` | Cross-platform, adds a dependency |

Option A is recommended: cloud-provider-kind only runs on Linux/macOS (same as Kind), so `pgrep` is always available. No new dependency.

**Installation:** No new `go get` required for core work. `mitchellh/go-ps` only if Option B is chosen.

---

## Architecture Patterns

### Recommended Project Structure

```
internal/deployer/
├── const.go                  # Add 4 new version vars + path constants
├── infrastructure.go         # Refactor: per-component functions + ComponentResult
├── infrastructure_test.go    # Tests for each isXxxReady() function
├── cloud_provider_kind.go    # Binary download + process start/detect logic (new file)
└── cloud_provider_kind_test.go
cmd/
└── setup.go                  # Collect ComponentResult slice, print per-line status
internal/constants/const.go   # Add KubeasyConfigDir, KindConfigPath, CloudProviderKindBinPath
```

### Pattern 1: ComponentResult Model

**What:** Each installer returns a `ComponentResult` instead of `error`. The caller always proceeds regardless of individual failures.

**When to use:** All 5 component installers.

```go
// Source: project convention (mirrors existing IsInfrastructureReadyWithClient pattern)
type ComponentStatus string

const (
    StatusReady    ComponentStatus = "ready"
    StatusNotReady ComponentStatus = "not-ready"
    StatusMissing  ComponentStatus = "missing"
)

type ComponentResult struct {
    Name    string
    Status  ComponentStatus
    Message string
}

// Per-component installer signature pattern:
func installNginxIngress(ctx context.Context, clientset kubernetes.Interface, dynamicClient dynamic.Interface) ComponentResult
func installGatewayAPI(ctx context.Context, clientset kubernetes.Interface, dynamicClient dynamic.Interface) ComponentResult
func installCertManager(ctx context.Context, clientset kubernetes.Interface, dynamicClient dynamic.Interface) ComponentResult
func ensureCloudProviderKind(ctx context.Context) ComponentResult

// Readiness check signature pattern (testable):
func isNginxIngressReadyWithClient(ctx context.Context, clientset kubernetes.Interface) (bool, error)
func isCertManagerReadyWithClient(ctx context.Context, clientset kubernetes.Interface) (bool, error)
func isGatewayAPICRDsInstalled(ctx context.Context, clientset kubernetes.Interface) (bool, error)
```

### Pattern 2: Two-Pass Apply with REST Mapper Rebuild

**What:** Apply CRD manifests, then rebuild the REST mapper before applying custom resources that use those CRDs.

**When to use:** Gateway API (GatewayClass requires `GatewayClass` CRD) and cert-manager (skipping custom resources until after controller is up — cert-manager applies in single file, but CRDs-first applies anyway).

```go
// Source: k8s.io/client-go/restmapper package (already in use in infrastructure.go)

// Pass 1: apply CRDs manifest
if err := kube.ApplyManifest(ctx, crdsManifest, "", mapper, dynamicClient); err != nil {
    return ComponentResult{Name: "gateway-api", Status: StatusNotReady, Message: err.Error()}
}

// Rebuild REST mapper so GatewayClass GVR is now discoverable
groups, err := restmapper.GetAPIGroupResources(clientset.Discovery())
if err != nil {
    return ComponentResult{Name: "gateway-api", Status: StatusNotReady, Message: err.Error()}
}
freshMapper := restmapper.NewDiscoveryRESTMapper(groups)

// Pass 2: apply GatewayClass using fresh mapper
if err := kube.ApplyManifest(ctx, gatewayClassManifest, "", freshMapper, dynamicClient); err != nil {
    return ComponentResult{Name: "gateway-api", Status: StatusNotReady, Message: err.Error()}
}
```

### Pattern 3: Kind Cluster Config with ExtraPortMappings

**What:** Create Kind cluster with port mappings using typed API. Write config to `~/.kubeasy/kind-config.yaml` for later audit.

**When to use:** New cluster creation in `setup.go`.

```go
// Source: pkg.go.dev/sigs.k8s.io/kind/pkg/apis/config/v1alpha4
// Source: pkg.go.dev/sigs.k8s.io/kind/pkg/cluster (CreateWithV1Alpha4Config)
import (
    kindv1alpha4 "sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
    "sigs.k8s.io/kind/pkg/cluster"
)

cfg := &kindv1alpha4.Cluster{
    TypeMeta: kindv1alpha4.TypeMeta{
        Kind:       "Cluster",
        APIVersion: "kind.x-k8s.io/v1alpha4",
    },
    Nodes: []kindv1alpha4.Node{
        {
            Role: kindv1alpha4.ControlPlaneRole,
            ExtraPortMappings: []kindv1alpha4.PortMapping{
                {ContainerPort: 80,  HostPort: 8080, Protocol: kindv1alpha4.PortMappingProtocolTCP},
                {ContainerPort: 443, HostPort: 8443, Protocol: kindv1alpha4.PortMappingProtocolTCP},
            },
        },
    },
}

provider.Create("kubeasy",
    cluster.CreateWithV1Alpha4Config(cfg),
    cluster.CreateWithNodeImage(constants.KindNodeImage),
)
```

### Pattern 4: Detach cloud-provider-kind as Background Process

**What:** Start binary as a completely detached OS process using `Setsid: true`.

**When to use:** cloud-provider-kind start after binary download.

```go
// Source: os/exec stdlib; syscall.SysProcAttr — standard Go Unix pattern
import (
    "os/exec"
    "syscall"
)

func startCloudProviderKindDetached(binPath string) error {
    cmd := exec.Command(binPath)
    cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
    cmd.Stdout = nil
    cmd.Stderr = nil
    cmd.Stdin = nil
    return cmd.Start()
    // cmd.Wait() is intentionally NOT called — process runs independently
}
```

### Pattern 5: Per-Component Status Output in setup.go

**What:** Collect `[]ComponentResult` and print one line per component using existing `ui` package primitives.

```go
// Source: existing ui package (ui.Success, ui.Warning, ui.Error)
func printComponentResult(r ComponentResult) {
    switch r.Status {
    case deployer.StatusReady:
        ui.Success(fmt.Sprintf("%s: ready", r.Name))
    case deployer.StatusNotReady:
        ui.Error(fmt.Sprintf("%s: not-ready — %s", r.Name, r.Message))
    case deployer.StatusMissing:
        ui.Warning(fmt.Sprintf("%s: missing — %s", r.Name, r.Message))
    }
}
```

### Anti-Patterns to Avoid

- **Monolithic `SetupInfrastructure()` returning a single error:** Prevents per-component idempotency and mixed-success scenarios. Replace with per-component functions.
- **Single-pass apply for Gateway API:** Applying CRDs and GatewayClass in one pass fails because the REST mapper doesn't know about `GatewayClass` until after the CRDs are applied.
- **Blocking on cloud-provider-kind:** Calling `cmd.Wait()` on the background process — the process must outlive the CLI. Use `cmd.Start()` only.
- **`cmd.Wait()` leak concern:** The parent (CLI) exits, orphaning the child intentionally. This is correct behavior — `Setsid` ensures the process is adopted by init.
- **Silent cluster recreation:** Always call `ui.Confirmation()` before `kind delete cluster` + recreate.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Kind cluster config with port mappings | Custom YAML string building | `kindv1alpha4.Cluster` struct + `CreateWithV1Alpha4Config` | Type-safe, already in go.mod |
| REST mapper rebuild | Custom discovery loop | `restmapper.GetAPIGroupResources` + `restmapper.NewDiscoveryRESTMapper` | Already used in `infrastructure.go` |
| Process detection (is binary running?) | Custom `/proc` scanning or shell parsing | `exec.Command("pgrep", "-f", "cloud-provider-kind")` | 3 lines, no dependency, correct for Unix targets |
| Deployment readiness polling | Custom wait loop | `kube.WaitForDeploymentsReady` (already exists) | Handles timeouts, NotFound, partial ready |
| Interactive confirmation | Custom stdin reading | `ui.Confirmation()` (already exists via pterm) | Consistent UX |

---

## Common Pitfalls

### Pitfall 1: cert-manager Webhook "Ready" but Not Actually Ready

**What goes wrong:** After `kube.WaitForDeploymentsReady()` returns for `cert-manager-webhook`, creating Certificate or Issuer resources immediately fails with "context deadline exceeded" or "connection refused."

**Why it happens:** The Kubernetes readiness probe for cert-manager-webhook reports `200 OK` while the TLS bootstrap between cert-manager and its webhook is still being established. The Endpoints object may show a pod IP, but the webhook TLS handshake isn't complete yet.

**How to avoid:** After `WaitForDeploymentsReady` succeeds, add a 15–30 s polling loop on the `cert-manager-webhook` Endpoints object — specifically waiting for `Subsets[0].Addresses` to be non-empty (not just ReadyReplicas). Add a fixed 15 s sleep or poll with 5 s intervals up to 30 s.

**Warning signs:** Tests that create an `Issuer` immediately after `WaitForDeploymentsReady` fail intermittently.

### Pitfall 2: FetchManifest URL Allowlist Rejection

**What goes wrong:** `kube.FetchManifest(url)` returns an error for the cert-manager or Gateway API URL if the URL doesn't start with an allowed prefix.

**Why it happens:** `fetchManifestAllowedPrefixes` currently only allows `https://github.com/` and `https://raw.githubusercontent.com/`. All new manifest URLs for this phase use these prefixes — this is already covered.

**How to avoid:** Verify each new manifest URL starts with `https://github.com/` (GitHub release downloads do). No allowlist changes needed.
- nginx-ingress: `https://raw.githubusercontent.com/kubernetes/ingress-nginx/...` ✓
- Gateway API: `https://github.com/kubernetes-sigs/gateway-api/releases/download/...` ✓
- cert-manager: `https://github.com/cert-manager/cert-manager/releases/download/...` ✓

### Pitfall 3: Gateway API Two-Pass Apply — GatewayClass Not in Namespace

**What goes wrong:** `ApplyManifest` fails when setting namespace on a cluster-scoped `GatewayClass` resource — or the resource ends up in the wrong scope.

**Why it happens:** `GatewayClass` is cluster-scoped. `ApplyManifest` correctly uses `mapping.Scope.Name()` to decide whether to set a namespace, so no override needed. However, if the passed `namespace` parameter is set to a namespace string, it will only be applied to namespaced resources.

**How to avoid:** Pass `""` (empty string) as the namespace when applying the GatewayClass manifest — `ApplyManifest` already handles scope detection. Verified in current implementation.

### Pitfall 4: cloud-provider-kind Binary Path and Permissions

**What goes wrong:** Downloaded binary is not executable — `cmd.Start()` returns "permission denied."

**Why it happens:** `http.Get` + `io.Copy` to file does not set execute permissions.

**How to avoid:** After writing binary to `~/.kubeasy/bin/cloud-provider-kind`, call `os.Chmod(binPath, 0755)` before `cmd.Start()`.

### Pitfall 5: Kind Cluster ExtraPortMappings — Existing Cluster Cannot Be Patched

**What goes wrong:** Existing `kubeasy` cluster was created without port mappings. nginx-ingress cannot receive traffic on host ports.

**Why it happens:** Kind cluster config is immutable after creation — port mappings cannot be added to a running cluster.

**How to avoid (per CONTEXT.md decision):** At setup time, read `~/.kubeasy/kind-config.yaml`. If it's missing or lacks `extraPortMappings`, prompt user for confirmation, then delete and recreate cluster. Never recreate silently.

### Pitfall 6: Detached Process Leaves Stale PID on macOS

**What goes wrong:** `pgrep -f cloud-provider-kind` returns a PID, but the process is a zombie or stale from a previous crashed run.

**Why it happens:** If the binary path changes (version upgrade) or the process crashed without cleanup, the name still matches `pgrep`.

**How to avoid:** After detecting a running process by name, optionally verify it's still alive by sending signal 0: `kill -0 <pid>` (zero-signal just checks existence). In Go: `syscall.Kill(pid, 0)`. If signal 0 fails → process is gone → download and restart.

---

## Code Examples

### Manifest URLs

```go
// Source: verified against GitHub release pages (2026-03-11)

func nginxIngressKindManifestURL() string {
    // Kind-specific manifest includes hostNetwork patch, tolerations, and ingressClassName=nginx
    return fmt.Sprintf(
        "https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-%s/deploy/static/provider/kind/deploy.yaml",
        NginxIngressVersion,
    )
}

func gatewayAPICRDsURL() string {
    // Standard channel — does NOT require server-side apply (only experimental channel does)
    return fmt.Sprintf(
        "https://github.com/kubernetes-sigs/gateway-api/releases/download/%s/standard-install.yaml",
        GatewayAPICRDsVersion,
    )
}

func certManagerCRDsURL() string {
    return fmt.Sprintf(
        "https://github.com/cert-manager/cert-manager/releases/download/%s/cert-manager.crds.yaml",
        CertManagerVersion,
    )
}

func certManagerInstallURL() string {
    return fmt.Sprintf(
        "https://github.com/cert-manager/cert-manager/releases/download/%s/cert-manager.yaml",
        CertManagerVersion,
    )
}
```

### cloud-provider-kind Binary Download URL

```go
// Source: https://api.github.com/repos/kubernetes-sigs/cloud-provider-kind/releases/latest
// Asset naming: cloud-provider-kind_{VERSION}_{OS}_{ARCH}.tar.gz
// VERSION format: "0.10.0" (no leading v in filename, unlike the git tag "v0.10.0")
// Supported: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64

func cloudProviderKindBinaryURL() (string, error) {
    goos := runtime.GOOS     // "linux", "darwin", "windows"
    goarch := runtime.GOARCH // "amd64", "arm64"

    // Strip leading "v" from version for asset filename
    version := strings.TrimPrefix(CloudProviderKindVersion, "v")

    return fmt.Sprintf(
        "https://github.com/kubernetes-sigs/cloud-provider-kind/releases/download/%s/cloud-provider-kind_%s_%s_%s.tar.gz",
        CloudProviderKindVersion, version, goos, goarch,
    ), nil
}
```

### GatewayClass Resource YAML

```go
// Source: cloud-provider-kind README (gatewayClassName confirmed as "cloud-provider-kind")
const gatewayClassManifest = `
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: cloud-provider-kind
spec:
  controllerName: sigs.k8s.io/cloud-provider-kind
`
```

### cert-manager Webhook Readiness Polling

```go
// Source: cert-manager GitHub issues #3045; cert-manager webhook debugging guide
// Poll Endpoints object (not just deployment ReadyReplicas) after deployment is ready

func waitForCertManagerWebhookEndpoints(ctx context.Context, clientset kubernetes.Interface) error {
    return wait.PollUntilContextTimeout(ctx, 5*time.Second, 60*time.Second, true,
        func(ctx context.Context) (bool, error) {
            ep, err := clientset.CoreV1().Endpoints("cert-manager").Get(
                ctx, "cert-manager-webhook", metav1.GetOptions{},
            )
            if err != nil {
                return false, nil // not found yet, retry
            }
            for _, subset := range ep.Subsets {
                if len(subset.Addresses) > 0 {
                    return true, nil
                }
            }
            return false, nil
        },
    )
}
```

### Process Detection for cloud-provider-kind

```go
// Source: os/exec stdlib — pgrep is available on macOS and Linux (same platforms Kind runs on)
// This is Claude's discretion — simple approach chosen over adding go-ps dependency

func isCloudProviderKindRunning() bool {
    cmd := exec.Command("pgrep", "-f", "cloud-provider-kind")
    return cmd.Run() == nil // exit 0 = found, exit 1 = not found
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Monolithic `SetupInfrastructure()` | Per-component functions with `ComponentResult` | This phase | Enables idempotency and partial success |
| cloud-provider-kind as user-installed daemon | Auto-downloaded + detached background process | CONTEXT.md decision | No user action required for LoadBalancer IPs |
| Single-pass REST mapper | Two-pass (CRDs first, rebuild, then apply custom resources) | Gateway API need | Enables GatewayClass registration after CRD install |
| Global infra ready check | Per-component `isXxxReadyWithClient()` | This phase | Enables component-level retry |
| No Kind cluster config file | `~/.kubeasy/kind-config.yaml` written at cluster creation | INFRA-06 | Enables detection of missing extraPortMappings |

**Deprecated/outdated:**
- `IsInfrastructureReady()` / `IsInfrastructureReadyWithClient()`: replaced by per-component `isNginxIngressReadyWithClient()`, `isCertManagerReadyWithClient()`, etc. The existing function may be kept for backward compat or removed — planner to decide.
- `SetupInfrastructure()`: replaced by `setupAllComponents()` or equivalent orchestrator + individual `installX()` functions.

---

## Open Questions

1. **cloud-provider-kind controllerName string**
   - What we know: GatewayClass `name` is `"cloud-provider-kind"` (confirmed from README)
   - What's uncertain: The `spec.controllerName` field value. README doesn't show the example GatewayClass spec fully. Common pattern is `sigs.k8s.io/<project>`.
   - Recommendation: Use `sigs.k8s.io/cloud-provider-kind` as controller name (standard kubernetes-sigs convention). If GatewayClass fails to be accepted by the controller, check actual controller name in cloud-provider-kind source code before planning.

2. **cert-manager v1.19.4 CRDs-only manifest vs combined manifest**
   - What we know: Both `cert-manager.crds.yaml` and `cert-manager.yaml` exist as separate assets
   - What's uncertain: Does `cert-manager.yaml` include the CRDs inline? If so, two-pass means: apply `cert-manager.yaml` (which includes CRDs embedded), wait for CRDs to be established, then no second pass needed. OR: apply `cert-manager.crds.yaml`, rebuild mapper, apply `cert-manager.yaml`.
   - Recommendation: Use two separate files (`cert-manager.crds.yaml` then `cert-manager.yaml`) to keep the pattern clean and consistent with Gateway API approach. The combined `cert-manager.yaml` includes CRDs, so there may be overlap — acceptable, idempotent.

3. **nginx-ingress port mapping — 8080/8443 vs 80/443**
   - What we know: Kind's ingress docs show 80/443 for extraPortMappings; CONTEXT.md specifies 8080/8443 for non-privileged ports
   - What's uncertain: The Kind-specific ingress-nginx manifest uses `hostPort: 80` and `hostPort: 443` in the DaemonSet. Using extraPortMappings of 8080→80 and 8443→443 bridges host non-privileged ports to container ports.
   - Recommendation: `extraPortMappings` maps host 8080 → container 80, host 8443 → container 443. This is the correct approach — no manifest modification needed.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | none — `go test ./...` via `task test:unit` |
| Quick run command | `task test:unit` |
| Full suite command | `task test` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| INFRA-01 | `isNginxIngressReadyWithClient` returns false when namespace/deployment missing | unit | `go test ./internal/deployer/ -run TestIsNginxIngressReady -v` | ❌ Wave 0 |
| INFRA-01 | `nginxIngressKindManifestURL()` embeds version, uses raw.githubusercontent.com prefix | unit | `go test ./internal/deployer/ -run TestNginxIngressURL -v` | ❌ Wave 0 |
| INFRA-02 | `isGatewayAPICRDsInstalled` returns false when CRDs absent | unit | `go test ./internal/deployer/ -run TestIsGatewayAPICRDsInstalled -v` | ❌ Wave 0 |
| INFRA-02 | `gatewayAPICRDsURL()` embeds version | unit | `go test ./internal/deployer/ -run TestGatewayAPIURL -v` | ❌ Wave 0 |
| INFRA-03 | `cloudProviderKindBinaryURL()` produces correct URL for linux/amd64 and darwin/arm64 | unit | `go test ./internal/deployer/ -run TestCloudProviderKindURL -v` | ❌ Wave 0 |
| INFRA-04 | `isCertManagerReadyWithClient` returns false when webhook deployment missing | unit | `go test ./internal/deployer/ -run TestIsCertManagerReady -v` | ❌ Wave 0 |
| INFRA-04 | cert-manager URL functions embed version, use github.com prefix | unit | `go test ./internal/deployer/ -run TestCertManagerURLs -v` | ❌ Wave 0 |
| INFRA-06 | Kind config YAML file is written to `~/.kubeasy/kind-config.yaml` with extraPortMappings | unit | `go test ./internal/deployer/ -run TestKindConfigWrite -v` | ❌ Wave 0 |
| INFRA-07 | `ComponentResult` with each status renders correct output | unit | `go test ./internal/deployer/ -run TestComponentResult -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `task test:unit`
- **Per wave merge:** `task test`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/deployer/infrastructure_test.go` — extend with per-component ready checks (INFRA-01, INFRA-02, INFRA-04, INFRA-07)
- [ ] `internal/deployer/cloud_provider_kind_test.go` — URL generation, binary path construction (INFRA-03)
- [ ] `internal/deployer/infrastructure_test.go` — Kind config file write test (INFRA-06)

---

## Sources

### Primary (HIGH confidence)
- `pkg.go.dev/sigs.k8s.io/kind/pkg/cluster` — `CreateWithV1Alpha4Config`, `CreateWithRawConfig`, `CreateWithConfigFile` confirmed
- `api.github.com/repos/kubernetes-sigs/cloud-provider-kind/releases/latest` — v0.10.0 asset naming confirmed: `cloud-provider-kind_{ver}_{os}_{arch}.tar.gz`
- `raw.githubusercontent.com/kubernetes-sigs/cloud-provider-kind/main/README.md` — GatewayClass name `"cloud-provider-kind"` confirmed
- Existing codebase: `internal/deployer/infrastructure.go`, `infrastructure_test.go`, `const.go`, `kube/client.go`, `kube/manifest.go`, `internal/ui/ui.go`, `cmd/setup.go` — all patterns read directly

### Secondary (MEDIUM confidence)
- nginx-ingress Kind manifest URL: `https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.15.0/deploy/static/provider/kind/deploy.yaml` — confirmed via WebSearch cross-reference with kind.sigs.k8s.io ingress docs
- Gateway API v1.2.1 standard-install.yaml: URL pattern confirmed from github.com/kubernetes-sigs/gateway-api/releases; standard channel does NOT require server-side apply (experimental channel does — confirmed via WebSearch)
- cert-manager v1.19.4: separate `cert-manager.crds.yaml` and `cert-manager.yaml` assets confirmed via artifacthub listing and WebSearch
- Process detach pattern `SysProcAttr{Setsid: true}`: confirmed via WebSearch with multiple Go sources
- REST mapper rebuild: `restmapper.GetAPIGroupResources` + `NewDiscoveryRESTMapper` — confirmed via pkg.go.dev

### Tertiary (LOW confidence)
- cloud-provider-kind `spec.controllerName: sigs.k8s.io/cloud-provider-kind` — inferred from kubernetes-sigs naming convention; not confirmed from README text
- cert-manager webhook 15–30 s post-Ready delay — confirmed as real phenomenon from GitHub issues; exact duration varies

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already in go.mod; manifest URLs cross-verified
- Architecture: HIGH — per-component pattern is a direct extension of existing `IsInfrastructureReadyWithClient` pattern
- Pitfalls: HIGH for cert-manager webhook and FetchManifest allowlist; MEDIUM for cloud-provider-kind stale PID

**Research date:** 2026-03-11
**Valid until:** 2026-04-11 (stable infrastructure components; cloud-provider-kind is active, check for v0.11.0 before release)
