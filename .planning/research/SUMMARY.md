# Project Research Summary

**Project:** kubeasy-cli v2.7.0 — Extended Connectivity Validation
**Domain:** Go CLI — Kubernetes connectivity validation extension (brownfield)
**Researched:** 2026-03-11
**Confidence:** HIGH

## Executive Summary

kubeasy-cli v2.7.0 is a focused brownfield extension to an established Go CLI tool. The core pattern is already proven: download a manifest from GitHub, apply it via `kube.ApplyManifest`, wait for readiness. This milestone extends that pattern to install four new infrastructure components (ingress-nginx, Gateway API CRDs, cert-manager, cloud-provider-kind advisory) and significantly expands the `connectivity` validation type with three new capabilities: auto-managed probe pods for NetworkPolicy testing, external HTTP checks from the CLI host for Ingress/Gateway validation, and TLS certificate inspection via Go stdlib. No new Go module dependencies are required — `crypto/tls`, `crypto/x509`, and `net/http` from the standard library cover all new Go-side functionality.

The recommended implementation strategy is to extend the existing `connectivity` validation type with a `mode` discriminant (`internal` | `external`) rather than introducing new `ValidationType` constants. This preserves backward compatibility with existing `challenge.yaml` files and the backend submission format. The architecture isolates probe pod lifecycle management in `deployer/probe.go` (keeping the validation executor cluster-read-only) and moves external HTTP logic to a new `validation/external.go` file to prevent `executor.go` from growing further. Infrastructure setup extends `SetupInfrastructure()` sequentially following the existing pattern, with one critical exception: a two-pass mapper refresh is mandatory between applying Gateway API CRDs and applying any GatewayClass resources.

The dominant risks are operational rather than algorithmic. cloud-provider-kind cannot be installed by the CLI — it is a host-side daemon that must run in a separate terminal, and on macOS the Docker network IPs it assigns are not directly routable from the host. The cert-manager webhook has a 15-30 second warm-up window after its deployment becomes ready, during which CR creation fails. The Kind cluster must be created with `extraPortMappings` for ports 80/443 before nginx-ingress can work — this cannot be patched after the fact. Each of these requires explicit handling (detection + actionable user messages), not silent failure.

## Key Findings

### Recommended Stack

The infrastructure additions all follow the existing fetch-manifest + apply pattern and require no new Go module dependencies. ingress-nginx is pinned to controller-v1.15.0 using the Kind-specific static manifest from the kubernetes org (the only controller with a Kind-maintained deploy.yaml). Gateway API CRDs are pinned to v1.2.1 Standard channel — v1.5.0 cannot be used because its CRDs exceed annotation size limits and require `--server-side=true`, which is incompatible with the current `ApplyManifest` Create/Update implementation. cert-manager v1.19.4 is the latest stable release. cloud-provider-kind v0.10.0 is the officially recommended solution for LoadBalancer IP assignment in Kind and additionally provides a built-in Gateway API controller (`kind.sigs.k8s.io/gateway-controller`), eliminating the need for a separate Gateway controller.

**Core technologies:**
- ingress-nginx controller-v1.15.0: HTTP/HTTPS routing — Kind-specific static manifest, no Helm needed
- Gateway API CRDs v1.2.1 (Standard channel): GatewayClass/Gateway/HTTPRoute CRDs — last version compatible with Create/Update apply semantics
- cert-manager v1.19.4: TLS certificate issuance for self-signed challenges — official single-file manifest
- cloud-provider-kind v0.10.0 (host binary): LoadBalancer IP assignment + built-in Gateway controller — documented only, not auto-installed
- `crypto/tls` + `crypto/x509` + `net/http` (Go stdlib): All TLS and external HTTP functionality — no new module imports

### Expected Features

All features in scope are classified P1 for the v2.7.0 milestone. The dependency graph is clear: probe pod infrastructure (PROBE-01/02/03) is the foundation for NetworkPolicy testing; external HTTP (EXT-01/02/03/04) and infrastructure setup (INFRA-01 through INFRA-06) together unlock Ingress and Gateway API challenges; TLS checks (TLS-01/02/03) are a low-complexity extension of the external HTTP transport.

**Must have (table stakes — v2.7.0 scope):**
- Auto-managed probe pod with cleanup (PROBE-01/03) — required for NetworkPolicy challenges where challenge pods lack curl
- Probe pod namespace configuration (PROBE-02/CONN-02) — required for cross-namespace NetworkPolicy scenarios
- Expected status 0 for blocked connections (CONN-01) — required for "deny" assertion in NetworkPolicy validation
- External HTTP from CLI with Host header (EXT-01/02) — required for Ingress/Gateway API challenge testing
- External IP auto-resolution from Ingress/Gateway resource (EXT-03) — prevents hardcoded IPs that break between cluster runs
- TLS certificate expiry, SAN, and insecureSkipVerify checks (TLS-01/02/03) — completes external HTTP story for cert-manager challenges
- wget fallback removal (PROBE-04) — closes deferred SEC-01 security debt, probe pod guarantees curl

**Infrastructure prerequisites:**
- Install ingress-nginx, Gateway API CRDs, cert-manager in `SetupInfrastructure()` (INFRA-01/02/03/04)
- `IsInfrastructureReady()` extended for new components (INFRA-06)
- cloud-provider-kind detection and actionable message (INFRA-05)

**Defer (future milestones):**
- HTTP/2 or gRPC connectivity — not needed for current challenge catalog
- UDP connectivity testing — requires NET_RAW privilege, out of scope
- mTLS validation — no planned challenges use it
- Real ACME/Let's Encrypt — impossible in local Kind environment

### Architecture Approach

The architecture is a surgical extension of two existing packages. `internal/deployer/` gains one new file (`probe.go` for `ProbePodManager`) and modifications to `infrastructure.go` and `const.go`. `internal/validation/` gains one new file (`external.go` for `executeExternal()`) and modifications to `executor.go`, `types.go`, and `loader.go`. No new top-level packages. The critical architectural decisions are: probe pod lifecycle lives in `deployer/`, not `validation/` (executor stays cluster-read-only); external HTTP is a separate file within `validation/` to keep executor.go bounded; and connectivity mode dispatch uses a `Mode` field on `ConnectivitySpec`, not a new `ValidationType`, to preserve backward compatibility.

**Major components:**
1. `internal/deployer/probe.go` (NEW) — `ProbePodManager`: create, wait-ready, cleanup probe pod; called from `cmd/submit.go` with deferred cleanup using independent background context
2. `internal/validation/external.go` (NEW) — `executeExternal()`: net/http client from CLI host, Host header, Ingress/Gateway IP resolution, TLS validation via crypto/tls
3. `internal/deployer/infrastructure.go` (MODIFIED) — extended `SetupInfrastructure()` with two-phase Gateway API CRD install, nginx-ingress, cert-manager, and webhook readiness polling
4. `internal/validation/types.go` (MODIFIED) — `ConnectivitySpec` extended with `Mode`, `Namespace`, `HostHeader`, TLS inline fields
5. `internal/validation/executor.go` (MODIFIED) — probe pod injection and mode dispatch in `executeConnectivity()`

### Critical Pitfalls

1. **Orphaned probe pods on Ctrl-C** — Use a separate `context.Background()`-derived context for pod deletion in defer; the validation context will already be cancelled. Also set `ActiveDeadlineSeconds` on the probe pod so it self-terminates if the CLI crashes.
2. **REST mapper stale after CRD installation** — Gateway API setup requires a two-pass approach: apply CRDs, call `restmapper.GetAPIGroupResources` to rebuild the mapper, then apply GatewayClass/controller resources. Single-pass will fail with "no matches for kind GatewayClass".
3. **cert-manager webhook timing** — Deployment Ready state is not sufficient. The webhook pod needs 15-30 seconds after reaching Ready to publish its serving certificate. Poll the `cert-manager-webhook` Endpoints object until non-empty before creating any cert-manager CRs.
4. **nginx-ingress requires extraPortMappings at cluster creation** — Ports 80/443 must be in the Kind cluster config when `kubeasy setup` creates the cluster. They cannot be added later. A `--reset` flag to delete and recreate the cluster is the recovery path.
5. **macOS LoadBalancer IPs are not routable from host** — Docker network IPs (172.18.x.x) assigned by cloud-provider-kind are inside a Linux VM on macOS. External HTTP validation must detect this condition and either fall back to NodePort/extraPortMappings or surface a clear actionable error rather than timing out silently.

## Implications for Roadmap

Based on the dependency graph from FEATURES.md and the build order from ARCHITECTURE.md, the natural grouping produces four phases:

### Phase 1: Type System and Infrastructure Foundation
**Rationale:** Types must compile before any other code can be written. Removing known security debt (wget) and extending the type system are prerequisite steps for all subsequent validation work. Infrastructure setup is independently parallelizable.
**Delivers:** Extended `ConnectivitySpec` and `ConnectivityCheck` types with `Mode`, `Namespace`, `HostHeader`, TLS fields; updated `loader.go` to accept new optional fields; wget fallback removed from executor; new version constants in `deployer/const.go`; infrastructure install steps for Gateway API CRDs, nginx-ingress, cert-manager; `IsInfrastructureReady()` extended.
**Addresses:** PROBE-04 (sec debt), INFRA-01 through INFRA-06
**Avoids:** REST mapper stale pitfall (two-pass CRD install from the start); nginx-ingress extraPortMappings pitfall (audit Kind config at this phase); cert-manager webhook timing pitfall (implement endpoint polling in this phase)

### Phase 2: Probe Pod Infrastructure (NetworkPolicy Testing)
**Rationale:** Probe pod is the foundation of internal connectivity testing. PROBE-01 and PROBE-03 must ship together — creating without cleanup is unacceptable. This phase also wires the submit command to the probe lifecycle.
**Delivers:** `deployer/probe.go` `ProbePodManager` with deterministic pod naming, background-context cleanup, `ActiveDeadlineSeconds`; `executeConnectivity()` extended with probe injection and namespace dispatch; `cmd/submit.go` wired with probe lifecycle (create before, deferred cleanup after `ExecuteAll`); CONN-01 (status 0 for blocked), CONN-02 (cross-namespace source).
**Addresses:** PROBE-01, PROBE-02, PROBE-03, CONN-01, CONN-02
**Avoids:** Orphaned probe pod pitfall (background context, ActiveDeadlineSeconds); parallel probe pod race (deterministic name per validation key or single pod reuse strategy decided here)

### Phase 3: External HTTP and IP Resolution
**Rationale:** External HTTP is the foundation for Ingress and Gateway API challenge testing. EXT-01 (transport) must exist before EXT-02 (Host header), EXT-03 (IP resolution), and EXT-04 (status code check) can be layered on. The macOS routing gap must be handled in EXT-03.
**Delivers:** `validation/external.go` with `executeExternal()`: `net/http` client with custom transport, Host header override via `req.Host`, Ingress IP resolution via `clientset.NetworkingV1().Ingresses()`, Gateway IP resolution via dynamic client, macOS Docker subnet detection with NodePort fallback or actionable error, expected status code comparison.
**Addresses:** EXT-01, EXT-02, EXT-03, EXT-04
**Avoids:** macOS LoadBalancer IP unreachable pitfall; `http.DefaultTransport` mutation (use per-request client)

### Phase 4: TLS Validation
**Rationale:** TLS checks share the external HTTP transport from Phase 3. They are low-complexity stdlib additions that complete the Ingress/Gateway story for cert-manager challenges.
**Delivers:** TLS configuration inline on `ConnectivityCheck`; `tls.Dial` with configurable `InsecureSkipVerify`; certificate expiry check via `cert.NotAfter`; SAN hostname check via `cert.DNSNames` and `cert.VerifyHostname`; `gosec G402` lint compliance (InsecureSkipVerify only on spec-opt-in path, never on DefaultTransport).
**Addresses:** TLS-01, TLS-02, TLS-03
**Avoids:** InsecureSkipVerify as default behavior; DefaultTransport mutation security pitfall

### Phase Ordering Rationale

- Type system first is non-negotiable — Go won't compile without types that downstream code references
- Infrastructure setup (Phase 1 concurrent track) is independent of validation code; it can be implemented alongside type work by a different contributor
- Probe pod (Phase 2) before external HTTP (Phase 3) because both modify `executeConnectivity()` — sequential prevents merge conflicts and keeps each PR reviewable
- TLS (Phase 4) last because it has zero dependencies except the Phase 3 transport, and it is the most self-contained unit
- PROBE-04 (wget removal) belongs in Phase 1 not Phase 2 — clean the known TODO before adding code around it

### Research Flags

Phases with well-documented patterns (skip research-phase):
- **Phase 1 (Type system + Infrastructure):** Extend existing types with optional fields — standard Go struct extension. Infrastructure install follows established `FetchManifest + ApplyManifest` pattern. Only the two-pass mapper and cert-manager webhook polling are non-obvious, both explicitly documented in research.
- **Phase 4 (TLS):** Go stdlib `crypto/tls` + `crypto/x509` usage is thoroughly documented. All fields needed (`NotAfter`, `DNSNames`, `VerifyHostname`) are confirmed in official pkg.go.dev docs.

Phases that may benefit from a brief implementation spike before full planning:
- **Phase 2 (Probe pod):** Concurrency model for multiple connectivity validations needs a concrete decision: single shared probe pod vs. per-validation pod with deterministic names. This affects naming strategy, `ExecuteAll` sequential vs. parallel behavior for connectivity type, and cleanup scope. Recommend a design decision document or spike before writing the phase plan.
- **Phase 3 (External HTTP + macOS):** The macOS Docker network routing gap has MEDIUM confidence — cloud-provider-kind v0.9+ is documented to address Ingress on macOS but the exact IP reachability behavior needs hands-on verification. Plan for a quick local test before committing to the NodePort fallback approach.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All versions verified against GitHub releases. Gateway API v1.2.1 vs v1.5.0 SSA constraint confirmed via official docs. Go stdlib coverage confirmed via pkg.go.dev. |
| Features | HIGH | Implementation patterns grounded in existing codebase source. Feature dependency graph explicitly verified. Anti-features well-reasoned with precedent from netassert, test-network-policies. |
| Architecture | HIGH | All findings grounded in live source code. Component boundaries match existing architectural conventions. Integration points confirmed in README and code comments. |
| Pitfalls | HIGH | cert-manager webhook timing confirmed via official troubleshooting docs. REST mapper limitation confirmed via controller-runtime issues and existing code comment. Kind extraPortMappings constraint confirmed via kind.sigs.k8s.io docs. |

**Overall confidence:** HIGH

### Gaps to Address

- **macOS external IP reachability:** MEDIUM confidence that cloud-provider-kind v0.10.0 resolves host routing on macOS. Recommend manual verification on macOS before finalizing EXT-03 implementation. If routing does not work, the NodePort/extraPortMappings fallback path becomes mandatory rather than optional.
- **NGINX Gateway Fabric vs ingress-nginx decision:** ARCHITECTURE.md recommends NGINX Gateway Fabric (NGF) as the forward-looking choice (ingress-nginx EOL announced for early 2026), but STACK.md recommends ingress-nginx controller-v1.15.0 as it has the Kind-specific static manifest. These two findings are in mild tension. Recommendation: use ingress-nginx for v2.7.0 (known working Kind manifest) and plan a migration to NGF in a follow-on milestone. This should be an explicit decision in the roadmap.
- **Parallel connectivity validation execution model:** `ExecuteAll` currently runs validations in parallel. Probe pod creation is stateful and must be serialized. The exact concurrency model (single shared probe pod, per-key deterministic pods, or sequential execution for connectivity type) is unresolved and must be decided in Phase 2 planning.
- **`kubeasy setup --reset` flag:** Pitfalls research identifies cluster recreation as the recovery path for missing `extraPortMappings`. Whether this flag already exists or needs to be added is not confirmed — requires checking `cmd/setup.go`.

## Sources

### Primary (HIGH confidence)
- [kubernetes/ingress-nginx Releases](https://github.com/kubernetes/ingress-nginx/releases) — controller-v1.15.0 confirmed
- [kubernetes-sigs/gateway-api Releases](https://github.com/kubernetes-sigs/gateway-api/releases) — v1.2.1 Standard channel verified; SSA requirement for v1.5.0 confirmed
- [kubernetes-sigs/cloud-provider-kind Releases](https://github.com/kubernetes-sigs/cloud-provider-kind/releases) — v0.10.0 external binary architecture confirmed
- [kind.sigs.k8s.io: LoadBalancer docs](https://kind.sigs.k8s.io/docs/user/loadbalancer/) — cloud-provider-kind recommended approach
- [kind.sigs.k8s.io: Ingress docs](https://kind.sigs.k8s.io/docs/user/ingress/) — extraPortMappings at cluster creation confirmed
- [cert-manager Releases](https://github.com/cert-manager/cert-manager/releases) — v1.19.4 latest confirmed
- [cert-manager webhook troubleshooting](https://cert-manager.io/docs/troubleshooting/webhook/) — webhook readiness timing confirmed
- [Go pkg.go.dev: crypto/tls](https://pkg.go.dev/crypto/tls) — InsecureSkipVerify, PeerCertificates confirmed
- [Go pkg.go.dev: crypto/x509](https://pkg.go.dev/crypto/x509) — DNSNames, IPAddresses, NotAfter, VerifyHostname confirmed
- Live codebase: `internal/validation/executor.go`, `internal/validation/types.go`, `internal/deployer/infrastructure.go`, `internal/deployer/const.go` — existing patterns confirmed

### Secondary (MEDIUM confidence)
- [NGINX Gateway Fabric manifest install docs](https://docs.nginx.com/nginx-gateway-fabric/install/manifests/open-source/) — NGF v2.4.x as forward-looking Ingress replacement
- [cert-manager + Gateway API transition announcement](https://cert-manager.io/announcements/2025/11/26/ingress-nginx-eol-and-gateway-api/) — ingress-nginx EOL context
- [Medium Nov 2025 — Gateway API with Kind and cloud-provider-kind](https://medium.com/@standardloop/nov-2025-using-k8s-gateway-api-with-kind-and-cloud-provider-kind-3a724b926780) — macOS routing behavior

### Tertiary (LOW confidence)
- macOS Docker network IP reachability with cloud-provider-kind v0.10.0 — inferred from Docker VM architecture; needs hands-on verification

---
*Research completed: 2026-03-11*
*Ready for roadmap: yes*
