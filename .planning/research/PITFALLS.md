# Pitfalls Research

**Domain:** Kubernetes CLI tool — connectivity validation extension (probe pods, Gateway API, cert-manager, TLS)
**Researched:** 2026-03-11
**Confidence:** HIGH (infrastructure pitfalls confirmed by official docs and GitHub issues; Go-specific from direct code analysis)

---

## Critical Pitfalls

### Pitfall 1: Orphaned Probe Pods on Ctrl-C

**What goes wrong:**
The CLI creates a temporary probe pod (curl image) for NetworkPolicy tests, then the user hits Ctrl-C. The context is cancelled, `defer` cleanup (pod deletion) is skipped because the deletion itself depends on the cancelled context, and the probe pod remains in the cluster indefinitely under a name like `kubeasy-probe-<uuid>`. On the next run a new pod is created. After several runs, the namespace fills with zombie pods.

**Why it happens:**
Go's `defer func() { clientset.CoreV1().Pods(ns).Delete(ctx, name, opts) }()` — where `ctx` is the validation context — fails silently when the context is already cancelled because the API call is rejected. Developers write the cleanup before thinking about context lifetime. The pattern looks correct but uses the wrong context.

**How to avoid:**
Use a separate background context for cleanup, never the validation context:
```go
cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cleanupCancel()
defer clientset.CoreV1().Pods(ns).Delete(cleanupCtx, podName, metav1.DeleteOptions{})
```
Register an `os/signal` handler in the executor that always runs cleanup before process exit. Also set `pod.Spec.RestartPolicy = corev1.RestartPolicyNever` and `ActiveDeadlineSeconds` so the pod self-terminates even if the CLI crashes.

**Warning signs:**
Running `kubectl get pods -n kubeasy-<challenge>` after Ctrl-C and seeing pods stuck in `Completed` or `Running` state named `kubeasy-probe-*`.

**Phase to address:**
PROBE-03 (probe pod cleanup) — must be implemented at the same time as PROBE-01 (creation), not as a follow-up phase.

---

### Pitfall 2: REST Mapper Stale After CRD Installation

**What goes wrong:**
`SetupInfrastructure()` already builds a `restmapper.NewDiscoveryRESTMapper` snapshot before applying manifests. The same pattern applied to Gateway API CRDs will fail: after applying the Gateway API CRD bundle, the existing REST mapper does not know about `gateway.networking.k8s.io/v1` resources. Any attempt to apply a `GatewayClass` or `Gateway` manifest in the same setup call returns `no matches for kind "GatewayClass"`.

**Why it happens:**
The REST mapper is a point-in-time snapshot of the API discovery endpoint taken before the CRDs exist. This is explicitly documented in the existing `infrastructure.go` comment: *"CRD types registered by the manifests applied below won't be resolvable within this call."* The existing code avoids the problem because Kyverno's install manifest only creates CRDs, not instances. Gateway API setup will need to create both CRDs and a GatewayClass, which breaks this assumption.

**How to avoid:**
Split setup into two separate mapper scopes: apply CRDs first, then rebuild the mapper, then apply instances (GatewayClass, Gateway). Pattern:
```go
// Phase 1: apply CRD bundle (no GatewayClass needed)
kube.ApplyManifest(ctx, crdManifest, ...)

// Phase 2: rebuild mapper after CRDs registered
groups, _ = restmapper.GetAPIGroupResources(clientset.Discovery())
mapper = restmapper.NewDiscoveryRESTMapper(groups)

// Phase 3: apply GatewayClass and controller manifests
kube.ApplyManifest(ctx, controllerManifest, ...)
```

**Warning signs:**
`no matches for kind "GatewayClass" in group "gateway.networking.k8s.io"` errors during setup, even though the CRD was just applied.

**Phase to address:**
INFRA-02 (Gateway API CRDs) and INFRA-03 (controller) — must explicitly handle mapper refresh between these two steps.

---

### Pitfall 3: cloud-provider-kind Must Run as External Process — Not Deployable by CLI

**What goes wrong:**
The CLI attempts to install cloud-provider-kind as a Kubernetes deployment or DaemonSet inside the Kind cluster. This fails because cloud-provider-kind is a **host-side binary** that runs outside the cluster. It must have privileges to manipulate Docker/Podman networks and assign IPs to host network interfaces. It cannot run as a container inside Kind.

**Why it happens:**
The name "cloud-provider" implies a cluster component. The tool's README is clear that it runs *"in a terminal"* as a standalone binary, but developers familiar with controllers expect it to live in the cluster.

**How to avoid:**
INFRA-05 must be documented, not automated. The CLI should:
1. Detect whether LoadBalancer IPs are being assigned (check service `.status.loadBalancer.ingress`)
2. If not assigned after a timeout, print an actionable message: `"External access requires cloud-provider-kind running on your host: run 'cloud-provider-kind' in a separate terminal."`
Do not attempt to install it. Do not block setup on it.

**Warning signs:**
`kubectl get svc` shows `<pending>` in EXTERNAL-IP column after installing an Ingress or Gateway controller with LoadBalancer type service. Services never receive IPs.

**Phase to address:**
INFRA-05 — the implementation decision here is "document, don't automate." This must be decided before writing setup code for nginx-ingress or Gateway API controllers.

---

### Pitfall 4: On macOS, LoadBalancer IPs Are Not Directly Reachable

**What goes wrong:**
Even with cloud-provider-kind running correctly, external IPs assigned to LoadBalancer services on macOS are Docker network IPs (e.g., `172.18.0.x`). These are not routable from the macOS host. The CLI's `net/http` client calls the LoadBalancer IP directly and gets connection refused or timeout, making external connectivity validation always fail on macOS.

**Why it happens:**
Docker on macOS runs inside a Linux VM. Container network interfaces are inside that VM, not exposed to the macOS host network. On Linux, Docker network IPs are directly accessible from the host. This is a macOS-specific Docker architecture constraint, not a Kubernetes bug.

**How to avoid:**
For external connectivity validation (EXT-01), resolve the IP from the Ingress/Gateway resource but also detect when the IP is a Docker subnet IP on macOS. Fall back to using `localhost` with the NodePort or extraPortMapping if the IP is not directly reachable. Alternatively, for Kind clusters, prefer extraPortMappings (configured at cluster creation time) over LoadBalancer IP resolution.

**Warning signs:**
`curl: (7) Failed to connect to 172.18.0.x port 80` on macOS. The IP is in Docker's bridge subnet range (172.17.0.0/16 or 172.18.0.0/16).

**Phase to address:**
EXT-03 (external IP/port resolution from Ingress/Gateway) — must handle macOS routing gap explicitly, not assume IP is reachable.

---

### Pitfall 5: cert-manager Webhook Not Ready When Issuer Resources Are Applied

**What goes wrong:**
`SetupInfrastructure()` applies the cert-manager install manifest and immediately applies a `ClusterIssuer` resource. The cert-manager webhook pod takes 15-30 seconds to generate its own self-signed serving certificate before it can validate custom resources. Applying `ClusterIssuer` too early returns `Error from server (InternalError): error when creating "issuer.yaml": Internal error occurred: failed calling webhook "webhook.cert-manager.io"`.

**Why it happens:**
cert-manager's admission webhook validates all cert-manager CRs. The webhook server generates its own TLS certificate at startup. The Kubernetes API server calls the webhook over mTLS. If the webhook pod has `Available=1` but hasn't yet published its serving certificate to its Secret, admission calls fail with connection refused or TLS handshake errors. `WaitForDeploymentsReady` passing is not sufficient — the webhook endpoint must actually be reachable.

**How to avoid:**
After waiting for cert-manager deployments to be ready, add an explicit readiness probe loop against the webhook endpoint before applying any cert-manager CRs:
```go
// Verify webhook endpoint has IPs before creating ClusterIssuer
// Use: kubectl get endpoints -n cert-manager cert-manager-webhook
// Wait until ENDPOINTS column is non-empty
```
Or use `cmctl check api` if the cert-manager CLI is available. A 30-second sleep is not reliable — poll the endpoint object instead.

**Warning signs:**
`failed calling webhook "webhook.cert-manager.io": Post "https://cert-manager-webhook.cert-manager.svc:443/validate"` errors immediately after cert-manager deployment appears ready.

**Phase to address:**
INFRA-04 (cert-manager) — readiness check must go beyond `WaitForDeploymentsReady` and include webhook endpoint polling.

---

### Pitfall 6: nginx-ingress on Kind Requires Cluster Created With extraPortMappings

**What goes wrong:**
The `kubeasy setup` command creates the Kind cluster, then later tries to expose nginx-ingress on port 80/443. But `extraPortMappings` are set at **cluster creation time** in the Kind config and cannot be added after the fact. The nginx-ingress controller is deployed with a NodePort service, but the host machine has no port binding to that NodePort.

**Why it happens:**
Kind's port mapping system is implemented as a Docker port binding at container creation time. Unlike a VM's network interface, you cannot dynamically add port bindings to a running Docker container without recreating it. Developers assume they can patch this after setup, but they cannot.

**How to avoid:**
`kubeasy setup` must embed the Kind cluster config with the correct `extraPortMappings` for nginx-ingress (80→containerPort 80, 443→containerPort 443) **before** cluster creation. Audit the existing `setup.go` to verify the Kind cluster config includes these mappings. If the cluster was created without them, the user must `kubeasy setup --reset` (delete and recreate the cluster).

**Warning signs:**
`curl localhost:80` returns connection refused after nginx-ingress is deployed. `kubectl get svc -n ingress-nginx` shows NodePort service but no port binding on the host.

**Phase to address:**
INFRA-01 (nginx-ingress) — must also audit and update the Kind cluster config in `setup.go` before writing infrastructure deployment code.

---

### Pitfall 7: Parallel ExecuteAll with Probe Pod Creation Creates Race on Pod Names

**What goes wrong:**
`ExecuteAll` runs all validations in parallel using goroutines. If two connectivity validations both need a probe pod and both use a generated name based on time or a hash of the spec, they may attempt to create pods with the same name and one creation fails, or both create distinct pods but the cleanup logic only removes one, leaving the other orphaned.

**Why it happens:**
Concurrent goroutines share no state coordination for probe pod naming. If pod names are deterministic (e.g., `kubeasy-probe-<challenge-slug>`), two goroutines create the same name and one gets a conflict error. If names are random (UUID), both pods are created but only one is used.

**How to avoid:**
Probe pod creation is a side-effectful, stateful operation — it must not run in parallel with other probe pod creations. Either: (a) use `ExecuteSequential` for connectivity validations that require probe pods, or (b) use a mutex per challenge namespace to serialize probe pod creation. Using deterministic names based on the validation key (e.g., `kubeasy-probe-<validation-key>`) prevents duplicate creation via a `GetOrCreate` pattern.

**Warning signs:**
`pods "kubeasy-probe-..." already exists` errors in test runs. Orphaned probe pods observed after validation of challenges with multiple connectivity checks.

**Phase to address:**
PROBE-01 (probe pod deployment) — naming strategy and concurrency model must be decided at design time, not as a fix later.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| `InsecureSkipVerify: true` globally for TLS validation | Eliminates CA cert loading complexity | golangci-lint `gosec G402` failure; security linter will block CI | Only for the `insecureSkipVerify: true` spec field path — must be opt-in per validation |
| Single REST mapper for entire setup call | Simpler code | Breaks when setup applies both CRDs and CR instances in one pass | Never acceptable for Gateway API setup — requires two-phase approach |
| Reuse cancelled context for probe cleanup | One context to manage | Cleanup silently no-ops; pods are orphaned | Never acceptable — cleanup always needs an independent context |
| `time.Sleep(30s)` after cert-manager install | Quick to implement | Flaky on slow machines, wasteful on fast ones | Never — use endpoint polling instead |
| `cloud-provider-kind` as optional undocumented step | Faster initial implementation | External validation silently always fails on macOS without it | Only acceptable if the CLI detects the failure and prints actionable guidance |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| cloud-provider-kind | Assume it's auto-started by Kind or kubectl | Detect `<pending>` EXTERNAL-IP and prompt user to start it manually |
| cert-manager ClusterIssuer | Create immediately after deploying cert-manager manifest | Poll `cert-manager-webhook` Endpoints until non-empty, then create Issuer |
| Gateway API CRDs | Apply standard-channel CRDs only | Some controllers (Cilium) require experimental CRDs for full reconciliation; check controller-specific requirements |
| nginx-ingress on Kind | Add extraPortMappings after cluster creation | Must be in Kind cluster config at creation time; cannot be patched dynamically |
| TLS validation in Go | Global `http.DefaultTransport` with InsecureSkipVerify | Create a per-request `http.Client` with a custom `Transport`; never mutate `http.DefaultTransport` |
| Probe pod exec (SPDY) | Ignore stderr on connectivity failure | SPDY exec returns exit code via error; parse stderr to distinguish "curl not found" from "connection refused" |
| Cross-namespace NetworkPolicy | Use pod labels from the challenge namespace on probe pod | Probe pod is in a different namespace; the NetworkPolicy `namespaceSelector` must match the probe pod's namespace label, not just `podSelector` |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Probe pod pending timeout absorbed into validation timeout | Validation reports "connection failed" instead of "probe pod never started" | Separate probe pod readiness wait from connectivity check timeout; fail fast if pod doesn't reach Running in N seconds | Any challenge where the probe image needs to be pulled for the first time (30-60s pull delay) |
| Re-pulling probe image on every validation run | 30-60s delay per run if image not cached | Use a common well-known image (e.g., `curlimages/curl:latest` or `alpine/curl:latest`) that users pull once during setup; pin a version tag | First run on a fresh cluster or after `docker system prune` |
| TLS certificate check on every `submit` call | Unnecessary TLS handshake latency for non-TLS challenges | Only instantiate TLS validation transport when `type: connectivity` and `tls:` block is present in spec | Challenges with many validations all running in parallel |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| `InsecureSkipVerify: true` as the default TLS behavior | All TLS validation passes trivially; cert-manager misconfiguration goes undetected | Default to strict TLS; require `insecureSkipVerify: true` to be explicitly set in the challenge spec (TLS-03) |
| Probe pod with `privileged: true` or broad RBAC | A misconfigured challenge could use the probe pod as a pivot point | Always create probe pods with `securityContext.allowPrivilegeEscalation: false`, no service account token, read-only root filesystem |
| wget fallback with `sh -c` still present in `checkConnectivity` | Shell injection via URL values in challenge.yaml (SEC-01 closed the curl path, not wget) | PROBE-04 removes wget fallback entirely — this is the correct resolution; probe pod always has curl |
| Using `http.DefaultTransport` for external validation | Leaks `InsecureSkipVerify` state between validation calls if concurrent | Always create a new `http.Client` per validation execution; never modify `http.DefaultTransport` |

---

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Generic "connectivity failed" message when probe pod is pending | User thinks their service is down; probe pod was never ready | Detect pod phase before exec; report "probe pod still Pending — image pull in progress" |
| Setup completes but cloud-provider-kind not running | External challenges silently fail with timeout; user has no idea why | After setup, check if any LoadBalancer services have `<pending>` IPs; print explicit actionable warning |
| cert-manager setup failure silently skipped | TLS challenges fail at validation time, not at setup time; confusing | If cert-manager setup fails, block setup completion with a clear error |
| TLS error "certificate signed by unknown authority" on first run | User unfamiliar with self-signed CA confusion | For `insecureSkipVerify: false` + self-signed cert, give explicit guidance: "Provide the CA cert via `caBundle` field or set `insecureSkipVerify: true`" |

---

## "Looks Done But Isn't" Checklist

- [ ] **Probe pod cleanup:** Often missing the background context for deletion — verify that deletion still occurs after context cancellation by testing Ctrl-C mid-validation.
- [ ] **Gateway API setup:** Often missing the mapper rebuild between CRD apply and GatewayClass apply — verify that `GatewayClass` is created without "no matches for kind" errors.
- [ ] **cert-manager readiness:** Often missing webhook endpoint polling — verify that `ClusterIssuer` creation succeeds immediately after `SetupInfrastructure` without retries.
- [ ] **nginx-ingress port binding:** Often missing `extraPortMappings` in Kind config — verify with `curl localhost:80` on the host before declaring INFRA-01 complete.
- [ ] **TLS validation:** Often missing the `x509.CertPool` loading for custom CA path — verify that a challenge with `insecureSkipVerify: false` and a self-signed cert actually fails before `caBundle` is provided.
- [ ] **MacOS external IP:** Often passes in CI (Linux) but fails locally — verify external connectivity validation on macOS explicitly, not just in GitHub Actions.
- [ ] **Parallel probe pod naming:** Often works in single-validation tests but breaks with multiple concurrent connectivity validations — verify with a challenge that has 2+ connectivity validations simultaneously.

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Orphaned probe pods | LOW | Add `kubectl delete pods -n <ns> -l app=kubeasy-probe` to `kubeasy challenge clean` |
| Stale REST mapper blocking GatewayClass creation | MEDIUM | Delete and rebuild the mapper object; wrap `ApplyManifest` calls in a retry with mapper refresh on "no matches" error |
| cert-manager webhook not ready | LOW | Retry `ClusterIssuer` creation after polling endpoint readiness; user-visible: `kubeasy setup` re-run is safe (idempotent) |
| Kind cluster missing extraPortMappings | HIGH | Cluster must be recreated; add `kubeasy setup --reset` flag that deletes and recreates the Kind cluster with correct config |
| cloud-provider-kind not running | LOW | Print actionable message; user starts it manually; no code change needed |
| InsecureSkipVerify in DefaultTransport | MEDIUM | Audit all `http.Client` creation in executor; add `gosec` G402 lint rule to CI to catch future occurrences |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Orphaned probe pods on Ctrl-C | PROBE-01 + PROBE-03 (implement together) | Kill CLI mid-validation; verify no orphaned pods in namespace |
| REST mapper stale after CRD install | INFRA-02 (Gateway API CRDs) | Apply GatewayClass immediately after CRD bundle; no "no matches" error |
| cloud-provider-kind external process | INFRA-05 (LoadBalancer IP) | Verify pending LoadBalancer detection and user-facing message |
| macOS LoadBalancer IP unreachable | EXT-03 (external IP resolution) | Run external connectivity validation on macOS with LoadBalancer service |
| cert-manager webhook timing | INFRA-04 (cert-manager) | Apply ClusterIssuer immediately after setup; no webhook error |
| nginx-ingress extraPortMappings | INFRA-01 (nginx-ingress) | `curl localhost:80` on host after setup; returns nginx default page |
| Parallel probe pod race | PROBE-01 (probe pod design) | Challenge with 2+ connectivity validations; no duplicate pod errors |
| InsecureSkipVerify in DefaultTransport | TLS-03 (insecureSkipVerify option) | `golangci-lint` G402 passes; DefaultTransport never modified |
| Probe pod exec wget fallback (existing debt) | PROBE-04 (remove wget fallback) | No `sh -c` in executor.go after probe pod is always available |
| Cross-namespace probe pod label mismatch | CONN-02 (cross-namespace source) | NetworkPolicy test with probe in separate namespace; traffic correctly blocked/allowed |

---

## Sources

- [cloud-provider-kind README — architecture, macOS limitations, external process requirement](https://github.com/kubernetes-sigs/cloud-provider-kind)
- [Kind LoadBalancer docs — extraPortMappings requirement, cloud-provider-kind integration](https://kind.sigs.k8s.io/docs/user/loadbalancer/)
- [Kind Ingress docs — extraPortMappings must be set at cluster creation](https://kind.sigs.k8s.io/docs/user/ingress/)
- [cert-manager webhook debugging guide — readiness timing, endpoint polling](https://cert-manager.io/docs/troubleshooting/webhook/)
- [cert-manager self-signed issuer docs — Subject DN empty by default](https://cert-manager.io/docs/configuration/selfsigned/)
- [kubernetes-sigs/cli-utils issue #421 — CRD + CR in same apply fails with caching RESTMapper](https://github.com/kubernetes-sigs/cli-utils/issues/421)
- [controller-runtime issue #321 — RESTMapper doesn't reflect new CRDs](https://github.com/kubernetes-sigs/controller-runtime/issues/321)
- [Go crypto/tls docs — InsecureSkipVerify, RootCAs, custom CA pool](https://pkg.go.dev/crypto/tls)
- [Datadog static analysis G402 — InsecureSkipVerify security rule](https://docs.datadoghq.com/security/code_security/static_analysis/static_analysis_rules/go-security/tls-skip-verify/)
- [kubernetes/kubernetes issue #38498 — orphaned pod cleanup](https://github.com/kubernetes/kubernetes/issues/38498)
- [Gateway API v1.1.0 standard-install breaks Envoy Gateway and Cilium — experimental CRDs may be required](https://github.com/kubernetes-sigs/gateway-api/issues/3075)
- [Medium Nov 2025 — Gateway API with Kind and cloud-provider-kind practical guide](https://medium.com/@standardloop/nov-2025-using-k8s-gateway-api-with-kind-and-cloud-provider-kind-3a724b926780)
- Direct analysis of `internal/validation/executor.go`, `internal/deployer/infrastructure.go` — existing patterns and known TODOs

---
*Pitfalls research for: kubeasy-cli v2.7.0 connectivity extension (probe pods, Gateway API, cert-manager, TLS)*
*Researched: 2026-03-11*
