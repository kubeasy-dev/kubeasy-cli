# Stack Research

**Domain:** Extended connectivity validation — Go CLI infrastructure additions
**Researched:** 2026-03-11
**Confidence:** HIGH (all versions verified against GitHub releases; architecture confirmed against live docs)

---

## Context

This is a brownfield milestone adding to an existing Go CLI (cobra, client-go, k8s.io/api v0.35.0).
The infrastructure pattern is already established: download manifest from GitHub → apply via `kube.ApplyManifest` → wait for readiness.
This research covers ONLY the net-new capabilities: ingress, Gateway API, cert-manager, LoadBalancer assignment, and TLS inspection.

---

## Recommended Stack

### Infrastructure Components (deployed into the Kind cluster)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| ingress-nginx (kubernetes/ingress-nginx) | controller-v1.15.0 | HTTP/HTTPS routing via Ingress resources | Only controller with a Kind-specific deploy manifest (`deploy/static/provider/kind/deploy.yaml`) that pre-configures NodePort and hostNetwork binding; no Helm needed; matches existing manifest-apply pattern |
| Gateway API CRDs (kubernetes-sigs/gateway-api) | v1.2.1 (Standard channel) | CRD definitions for GatewayClass, Gateway, HTTPRoute, TLSRoute | v1.2.1 is the last stable release shipping CRDs without requiring `--server-side=true`; v1.5.0 CRDs require server-side apply due to annotation size — incompatible with the current `ApplyManifest` implementation that uses `resourceClient.Create/Update`, not SSA |
| cert-manager | v1.19.4 | TLS certificate issuance (SelfSigned, CA, ACME issuers) | Latest stable as of March 2026; single-file install manifest from GitHub matches existing pattern; required for TLS-01/TLS-02/TLS-03 validation spec |
| cloud-provider-kind | v0.10.0 (external binary) | LoadBalancer IP assignment for Kind services; built-in Gateway API controller | Provides `kind.sigs.k8s.io/gateway-controller` GatewayClass automatically; fixes Ingress on Mac/Windows; the ONLY supported approach for LoadBalancer in Kind (MetalLB is an alternative but heavier) |

### Go Standard Library (no new imports)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `crypto/tls` | stdlib (Go 1.25.4) | TLS dial, certificate access, `InsecureSkipVerify` | All TLS connection setup in external connectivity checks |
| `crypto/x509` | stdlib (Go 1.25.4) | Certificate parsing, SAN inspection, expiry check | Inspecting `tls.ConnectionState().PeerCertificates[0]` for TLS-01/TLS-02 |
| `net/http` | stdlib (Go 1.25.4) | External HTTP requests from CLI directly | EXT-01 through EXT-04: external connectivity mode, Host header override, status code check |

No new Go module dependencies are required for the TLS inspection or external HTTP capabilities.

---

## Installation

```bash
# No new go get calls needed — stdlib only for Go-side additions.
# Infrastructure versions are defined as var constants in internal/deployer/const.go
# following the existing Renovate annotation pattern.

# Manifest URLs (for reference — fetched at runtime by FetchManifest):
# ingress-nginx Kind manifest:
#   https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.15.0/deploy/static/provider/kind/deploy.yaml

# Gateway API CRDs (Standard channel, no server-side apply required):
#   https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.1/standard-install.yaml

# cert-manager:
#   https://github.com/cert-manager/cert-manager/releases/download/v1.19.4/cert-manager.yaml
```

---

## Alternatives Considered

| Recommended | Alternative | Why Not |
|-------------|-------------|---------|
| ingress-nginx controller-v1.15.0 | Traefik, Contour, HAProxy | ingress-nginx is the only controller with a Kind-specific static manifest maintained by the kubernetes org; others require Helm or cluster-specific config; ingress-nginx is EOL in March 2026 but remains the only controller with a Kind-specific deploy manifest, and Kubeasy controls the version pin |
| cloud-provider-kind v0.10.0 (external binary) | MetalLB | cloud-provider-kind is the officially recommended Kind solution (kind.sigs.k8s.io docs); it bundles a Gateway API controller at no extra cost; MetalLB does not provide Gateway API and requires separate config CRDs |
| Gateway API CRDs v1.2.1 | v1.5.0 | v1.5.0 requires `kubectl apply --server-side=true` due to CRD annotation size; the existing `ApplyManifest` implementation uses Create/Update, not SSA; v1.2.1 is the latest version whose standard-install.yaml applies cleanly without SSA |
| stdlib `crypto/tls` + `net/http` | `github.com/hashicorp/go-retryablehttp`, third-party TLS libs | stdlib is sufficient for the required features (TLS dial, SAN check, expiry check, Host header, status code); adding a dependency for functionality that stdlib covers well would violate the project's minimal-dependency philosophy |
| Envoy Gateway v1.7.0 (not recommended — see below) | cloud-provider-kind built-in Gateway | cloud-provider-kind v0.9+ already provides a Gateway API controller (`kind.sigs.k8s.io/gateway-controller`) as its built-in; installing Envoy Gateway on top would add a second GatewayClass and a heavy operator |

---

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| Gateway API CRDs v1.5.0 | Requires `--server-side=true` (SSA) for `kubectl apply`; `ApplyManifest` uses Create/Update semantics, not SSA; applying v1.5.0 manifests will fail with annotation too long errors | Gateway API CRDs v1.2.1 Standard channel |
| Envoy Gateway (standalone) | cloud-provider-kind v0.9+ already ships a built-in Gateway API controller; installing Envoy Gateway adds a second competing GatewayClass, an operator with significant memory overhead (~200MB), and complicates cluster setup for a local learning tool | cloud-provider-kind's built-in Gateway controller |
| F5/NGINX (nginxinc/kubernetes-ingress) | No Kind-specific static manifest; requires Helm or complex NodePort configuration; different annotation schema from kubernetes/ingress-nginx | kubernetes/ingress-nginx with Kind static manifest |
| Cilium as a CNI replacement | Would require replacing Kind's default CNI (kindnet); high operational complexity; Cilium's Gateway API support is excellent in production but overkill for a local learning CLI | cloud-provider-kind built-in Gateway API |
| Helm inside the Go CLI | Project does not use Helm for any existing infrastructure; adding Helm client library adds significant binary size and complexity; all existing components use static manifest apply | `kube.FetchManifest` + `kube.ApplyManifest` pattern |

---

## Stack Patterns by Variant

**For TLS validation (TLS-01, TLS-02, TLS-03):**
- Use `crypto/tls.Dial` to establish the connection and retrieve `ConnectionState().PeerCertificates`
- Use `cert.NotAfter.After(time.Now())` for expiry (TLS-01)
- Use `cert.VerifyHostname(hostname)` or inspect `cert.DNSNames` + `cert.IPAddresses` for SAN check (TLS-02)
- Set `tls.Config{InsecureSkipVerify: true}` when `insecureSkipVerify: true` in spec (TLS-03)
- No external library needed — `crypto/x509.Certificate` struct exposes `DNSNames []string`, `IPAddresses []net.IP`, `NotAfter time.Time` directly

**For external HTTP validation (EXT-01 through EXT-04):**
- Use `net/http.Client` with a custom `Transport` (for TLS config) and no redirect following
- Set Host header via `req.Host = spec.HostHeader` (not `req.Header.Set("Host", ...)` — the latter is ignored by net/http)
- Resolve target IP from Ingress `.status.loadBalancer.ingress[0].ip` or Gateway `.status.addresses[0].value` via existing `client-go` dynamic client — no new library needed

**For probe pod management (PROBE-01 through PROBE-04):**
- Use existing `clientset.CoreV1().Pods()` CRUD — no new library
- Use `curlimage: curlimages/curl:8.11.1` (pinned, reproducible, ~6MB compressed) for the probe pod container
- Probe pod uses `restartPolicy: Never` and `command: ["sleep", "3600"]` so it stays alive for exec calls

**For IP/port resolution from Ingress/Gateway resources (EXT-03):**
- Use `dynamicClient.Resource(ingressGVR)` to fetch Ingress or use `clientset.NetworkingV1().Ingresses()` — already available in executor
- For Gateway, use dynamic client with GVR `{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"}`

---

## Version Compatibility

| Component | Compatible With | Notes |
|-----------|-----------------|-------|
| ingress-nginx controller-v1.15.0 | Kubernetes 1.25+ (Kind v0.31.0 ships 1.31) | Kind v0.31 ships k8s 1.31; ingress-nginx 1.15.x supports 1.28–1.31 |
| Gateway API CRDs v1.2.1 | Kubernetes 1.26+ | v1.2.1 requires k8s 1.26+; Kind 1.31 satisfies this |
| cert-manager v1.19.4 | Kubernetes 1.26+ | Patch release fixing CVE-2026-24051; requires k8s 1.26+ |
| cloud-provider-kind v0.10.0 | Kind v0.20.0+ | Must run as external binary on the host; requires Docker socket access; on Mac, Ingress fix is included in v0.10.0 |
| `crypto/tls` + `crypto/x509` | Go 1.25.4 | Already available; no version constraint |
| k8s.io/api v0.35.0 (existing) | All above components | Gateway API types are accessed via dynamic client (unstructured), not typed — no gateway-api Go module needed |

---

## Critical Integration Notes

### cloud-provider-kind requires a host-side binary, not a cluster deployment

cloud-provider-kind is **not deployed as a pod inside the cluster**. It runs as a binary on the developer's machine and monitors the cluster via its kubeconfig. It launches Docker containers (named `kindccm-*`) for each LoadBalancer service.

**Implication for `kubeasy setup`:** The CLI cannot install cloud-provider-kind automatically the same way it installs Kyverno. Instead:
- `kubeasy setup` should check if `cloud-provider-kind` binary is present on PATH
- If absent, print installation instructions: `go install sigs.k8s.io/cloud-provider-kind@v0.10.0` or download binary from GitHub releases
- Document that the user must keep `cloud-provider-kind` running in a terminal while using Ingress/Gateway features
- This satisfies INFRA-05 (document rather than auto-install)

### Gateway API CRDs: Standard vs Experimental channel

Standard channel (`standard-install.yaml`) includes: GatewayClass, Gateway, HTTPRoute, ReferenceGrant.
Experimental channel adds: GRPCRoute, TCPRoute, TLSRoute (v1 GA in v1.5.0 only), BackendTLSPolicy.

For the v2.7.0 scope (HTTP connectivity + TLS), **Standard channel is sufficient**. Install:
```
https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.1/standard-install.yaml
```

### `ApplyManifest` two-pass limitation for CRDs

The existing `ApplyManifest` builds the REST mapper once before applying all documents. CRDs registered by the first manifest won't be available to mapper within the same call. This is already documented in the code for Kyverno. cert-manager's install.yaml includes both CRDs and controller resources in one file — this requires a **two-pass approach** for cert-manager: first apply CRDs, then apply the remaining resources after a short wait (same pattern as would be needed for Gateway API CRD instances).

Gateway API CRDs `standard-install.yaml` contains **only CRDs** — no controller instances. This applies cleanly in one pass. The cloud-provider-kind binary (external) watches for and handles Gateway/HTTPRoute objects automatically once CRDs are present.

### `fetchManifestAllowedPrefixes` must be extended

`cert-manager.yaml` and Gateway API CRDs come from `https://github.com/cert-manager/cert-manager/...` and `https://github.com/kubernetes-sigs/gateway-api/...` — these are already covered by the existing `"https://github.com/"` prefix in the allowlist. The ingress-nginx Kind manifest comes from `https://raw.githubusercontent.com/kubernetes/ingress-nginx/...` — also already covered by `"https://raw.githubusercontent.com/"`. **No allowlist changes needed.**

---

## Sources

- [kubernetes/ingress-nginx Releases](https://github.com/kubernetes/ingress-nginx/releases) — controller-v1.15.0 confirmed latest (Mar 9, 2025)
- [Kubernetes Blog: Ingress NGINX Retirement](https://kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/) — EOL confirmed November 2025 announcement
- [kubernetes-sigs/gateway-api Releases](https://github.com/kubernetes-sigs/gateway-api/releases) — v1.2.1 Standard channel verified; v1.5.0 SSA requirement confirmed
- [kubernetes-sigs/cloud-provider-kind Releases](https://github.com/kubernetes-sigs/cloud-provider-kind/releases) — v0.10.0 latest confirmed; external binary architecture confirmed
- [kind.sigs.k8s.io: LoadBalancer docs](https://kind.sigs.k8s.io/docs/user/loadbalancer/) — cloud-provider-kind as recommended approach
- [kind.sigs.k8s.io: Ingress docs](https://kind.sigs.k8s.io/docs/user/ingress/) — ingress-nginx Kind manifest URL pattern confirmed
- [cert-manager Releases](https://github.com/cert-manager/cert-manager/releases) — v1.19.4 latest confirmed (CVE patch)
- [Envoy Gateway Releases](https://github.com/envoyproxy/gateway/releases) — v1.7.0 latest (Feb 2026); not recommended for this use case
- [Go pkg.go.dev: crypto/tls](https://pkg.go.dev/crypto/tls) — stdlib TLS capabilities confirmed
- [Go pkg.go.dev: crypto/x509](https://pkg.go.dev/crypto/x509) — `DNSNames`, `IPAddresses`, `NotAfter`, `VerifyHostname` confirmed available
- Existing codebase: `internal/deployer/infrastructure.go`, `internal/kube/manifest.go`, `internal/deployer/const.go` — pattern confirmed

---

*Stack research for: kubeasy-cli v2.7.0 — Extended Connectivity Validation*
*Researched: 2026-03-11*
