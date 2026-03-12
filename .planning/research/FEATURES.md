# Feature Research

**Domain:** Kubernetes connectivity validation extension for a CLI-based learning tool
**Researched:** 2026-03-11
**Confidence:** HIGH (implementation patterns are well-established; Go stdlib is authoritative)

---

## Context: What Already Exists

The existing `connectivity` validation type uses pod-exec to run `curl` from a user-designated source pod. The executor calls `kubectl exec`-equivalent (SPDY remotecommand) and reads the HTTP status code from stdout.

**Gap being addressed:** Challenge pods often have no networking tools installed. NetworkPolicy testing requires a controlled source pod that the CLI owns. Ingress/Gateway API testing cannot work via pod-exec — the CLI must make HTTP requests directly. TLS verification is missing entirely.

---

## Feature Landscape

### Table Stakes (Users Expect These)

Features that users expect once the connectivity validation is described as supporting NetworkPolicy, Ingress, and TLS. Missing these = the feature set is broken or incomplete.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Auto-managed probe pod | Without this, NetworkPolicy tests break on any challenge pod that lacks curl. Tooling like test-network-policies and netassert universally use a dedicated test pod. | MEDIUM | CLI creates a `kubeasy-probe` pod with curl, uses it for exec, deletes after. Namespace must be configurable to test cross-namespace policies. |
| Probe pod cleanup after validation | An unmanaged probe pod would pollute the cluster permanently. Cleanup is non-negotiable. | LOW | Delete pod after ExecuteAll returns (or on context cancellation). defer pattern in Go. |
| Expected status code = 0 for blocked connections | NetworkPolicy testing requires asserting that a connection is *refused* or *timed out*. Status 0 is the natural sentinel — curl returns empty/000 on connection failure. The type already documents `// Use 0 to verify connection failed`. | LOW | Change the executor comparison: `code == 0` means "connection failed" which is a passing result. Currently the `errNoSourcePodSpecified` branch blocks this. The types.go comment already anticipates it. |
| External HTTP from CLI via net/http | Ingress and Gateway API expose services via a LoadBalancer IP on the host network. Pod-exec cannot reach these — the CLI process must make the request. This is how every Kubernetes conformance test verifies ingress routing. | MEDIUM | `net/http` client with custom transport. Resolve IP from Ingress `.status.loadBalancer.ingress[0].ip` or Gateway `.status.addresses[0].value`. |
| Custom Host header for Ingress/Gateway routing | nginx-ingress and Gateway API controllers route by the `Host` header. Without this, all requests would fail — the IP alone does not route correctly in virtual-host setups. This is a required companion to external HTTP. | LOW | `req.Host = spec.Host` on the `http.Request`. One field. |
| TLS: certificate not expired | Any TLS validation that doesn't check expiry is incomplete. cert-manager issues certs locally; they can be short-lived. `x509.Certificate.NotAfter` is the authoritative field. | LOW | `time.Now().After(cert.NotAfter)` — two lines. |
| TLS: hostname matches SANs | A cert with the wrong SAN causes real connection failures. `crypto/tls` returns an error automatically unless `InsecureSkipVerify` is true. When skip=true, the CLI must manually check `cert.DNSNames`. | LOW | Parse `tls.ConnectionState().PeerCertificates[0].DNSNames` and compare against expected hostname. |
| TLS: insecureSkipVerify option | Self-signed certificates from a SelfSigned cert-manager Issuer will not be trusted by the default CA bundle. Without this option, every TLS challenge using a local CA would fail the dial itself. | LOW | `tls.Config{InsecureSkipVerify: true}` in the HTTP transport. Explicitly a validation tool — the security warning is acceptable in this context. |

### Differentiators (Competitive Advantage)

Features that add meaningful teaching value beyond what is strictly required.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Source pod namespace configurable | Enables teaching cross-namespace NetworkPolicy scenarios (deny by default, allow specific namespaces). Without it, all tests run from the challenge namespace — same namespace traffic is almost always allowed by default. | LOW | Add `Namespace` field to `SourcePod` struct. Executor uses `spec.SourcePod.Namespace` if set, falls back to `e.namespace`. |
| External IP/port resolution from Ingress/Gateway resource | Challenge authors write `ingressName: my-ingress` or `gatewayName: my-gateway`, and the CLI resolves the external IP automatically. Without auto-resolution, authors hardcode IPs, which change per cluster run. | MEDIUM | Fetch Ingress via `clientset.NetworkingV1().Ingresses().Get()`, read `.status.loadBalancer.ingress[0].ip`. For Gateway: dynamic client on `gateway.networking.k8s.io/v1`, read `.status.addresses[0].value`. |
| wget fallback removal (curl-only probe pod) | The probe pod uses a known image (e.g., `curlimages/curl`) — wget fallback is unnecessary and carries the `sh -c` shell injection risk (TODO(sec) in current code). Removing it closes SEC-01's deferred item. | LOW | Delete the wget block from `checkConnectivity`. The probe pod guarantees curl is present. |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| ICMP/ping connectivity tests | Intuitive for "is host reachable?" debugging | NetworkPolicies apply to TCP/UDP, not ICMP by default in most CNI plugins. ping results are misleading — a pod may accept ping but block HTTP. Teaches wrong mental model. | Use HTTP/TCP curl with short timeout. Status 0 = blocked is cleaner. |
| Reuse challenge pod as source (install curl into it) | Avoids probe pod lifecycle management | NetworkPolicy testing requires the source pod to have specific labels/namespace. Pollutes challenge resource; breaks if policy blocks pod modification. Requires curl in every challenge image — impossible to guarantee. | Auto-managed probe pod with known labels. |
| Ephemeral container injection into existing pods | netassert v2 uses this approach; avoids separate pod | Requires `ephemeralcontainers` subresource RBAC; NET_RAW capability for UDP; not stable in older clusters; adds complexity for HTTP-only use case. Kind clusters may not enable this by default. | Standalone probe pod: simpler, no privilege escalation. |
| HTTP/2 or gRPC connectivity checks | Modern services use these protocols | HTTP/2 requires TLS negotiation; gRPC needs proto definitions. Massively increases scope. Not taught in beginner Kubernetes challenges. | Stay with HTTP/1.1. TLS is already in scope; H2 is not. |
| Real ACME/Let's Encrypt certificate validation | Students might want "real" TLS | Requires external DNS + internet access. Kind clusters run locally. Let's Encrypt cannot reach them. | cert-manager SelfSigned + insecureSkipVerify is the correct local pattern. |
| Parallel probe pod creation per target | Speed improvement | Race condition on probe pod name; multiple pods leave cleanup complexity; adds overhead for the common single-target case. | Single probe pod, sequential target checks. Acceptable for a learning tool. |

---

## Feature Dependencies

```
[PROBE-01: Auto probe pod]
    └──required-by──> [CONN-01: status 0 for blocked]
    └──required-by──> [PROBE-02: configurable namespace]
    └──required-by──> [CONN-02: cross-namespace source]
    └──enables──>     [PROBE-04: remove wget fallback]

[PROBE-03: Probe pod cleanup]
    └──same-delivery-as──> [PROBE-01]

[INFRA-05: cloud-provider-kind LoadBalancer IPs]
    └──required-before──> [EXT-03: resolve IP from Ingress/Gateway]
    └──required-before──> [EXT-01: external HTTP from CLI]

[EXT-01: external HTTP from CLI]
    └──required-by──> [EXT-02: custom Host header]
    └──required-by──> [EXT-03: IP/port auto-resolution]
    └──required-by──> [EXT-04: expected status code check]
    └──required-by──> [TLS-01: cert not expired]
    └──required-by──> [TLS-02: hostname SANs]
    └──required-by──> [TLS-03: insecureSkipVerify]

[INFRA-01: nginx-ingress] ──enables──> [EXT-03: Ingress IP resolution]
[INFRA-02: Gateway API CRDs + INFRA-03: controller] ──enables──> [EXT-03: Gateway IP resolution]
[INFRA-04: cert-manager] ──enables──> [TLS-01/02/03: TLS checks]
```

### Dependency Notes

- **PROBE-01 must ship before CONN-01:** status=0 means "no source pod needed" OR "probe pod blocked" — distinguishing these requires the probe pod to exist and be running before the check fires.
- **INFRA-05 (cloud-provider-kind) must be documented/integrated before EXT-01:** Without LoadBalancer IP assignment, Ingress `.status.loadBalancer.ingress` stays empty and IP resolution fails silently.
- **EXT-01 is the foundation of the entire external + TLS feature set:** All six EXT and TLS items share a single `net/http` transport layer. Build it once, extend it.
- **PROBE-04 (remove wget) conflicts with keeping the old fallback:** These are mutually exclusive paths. Once probe pod is guaranteed, the fallback must be deleted — not conditionally skipped.

---

## MVP Definition

### Launch With (v2.7.0 — all in scope)

The milestone targets all items below. They form a coherent, minimal set where each is independently useful:

- [x] PROBE-01 + PROBE-03: Auto-managed probe pod with cleanup — unlocks NetworkPolicy testing
- [x] PROBE-02: Configurable probe namespace — unlocks cross-namespace scenarios
- [x] CONN-01: Expected status 0 support — makes NetworkPolicy "deny" assertions work
- [x] CONN-02: Source pod namespace — enables cross-namespace source for existing challenge pods
- [x] PROBE-04: Remove wget fallback — closes deferred SEC-01 item, simplifies executor
- [x] EXT-01 + EXT-02 + EXT-03 + EXT-04: External HTTP with Host header, auto-resolved IP — unlocks Ingress and Gateway API challenges
- [x] TLS-01 + TLS-02 + TLS-03: Certificate checks via crypto/tls — completes the external HTTP story

### Add After Validation (future)

- [ ] HTTP/2 connectivity support — only if challenges require it; current challenges do not
- [ ] Metrics from ingress (latency, error rate) — VTYPE-03 in backlog
- [ ] RBAC validation type — VTYPE-01 in backlog; unrelated to connectivity

### Future Consideration (v3+)

- [ ] UDP connectivity testing — requires NET_RAW; out of scope for learning tool
- [ ] mTLS validation — mutual TLS adds cert rotation complexity; no planned challenges use it
- [ ] Real ACME cert validation — impossible in local Kind environment

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Auto probe pod (PROBE-01/03) | HIGH — unblocks NetworkPolicy challenges entirely | MEDIUM — pod lifecycle, name collision, cleanup | P1 |
| Status 0 for blocked (CONN-01) | HIGH — required for "deny" assertions | LOW — single comparison change | P1 |
| Probe namespace (PROBE-02/CONN-02) | HIGH — cross-namespace policies need it | LOW — one field on SourcePod | P1 |
| External HTTP (EXT-01/02/04) | HIGH — required for Ingress challenges | MEDIUM — net/http transport, Host header | P1 |
| IP auto-resolution (EXT-03) | HIGH — challenge portability without hardcoded IPs | MEDIUM — Ingress + Gateway API fetch logic | P1 |
| TLS checks (TLS-01/02/03) | MEDIUM — cert-manager challenges benefit greatly | LOW — crypto/tls fields are trivial to read | P1 |
| Remove wget fallback (PROBE-04) | MEDIUM — security hygiene, test simplification | LOW — delete dead code | P1 |
| Infrastructure setup (INFRA-01..06) | HIGH — prerequisite for external tests | MEDIUM — 4 separate components | P1 (prerequisite) |

**Priority key:**
- P1: Must have for launch (all items here are P1 — they are the milestone scope)

---

## Ecosystem Reference

### How NetworkPolicy Testing Works in Practice

Standard pattern (used by test-network-policies, netassert v1, kubetest):
1. Deploy a controlled "source" pod with known labels (e.g., `kubeasy-probe`).
2. Apply the NetworkPolicy under test.
3. Exec `curl` from the source pod to the target service.
4. Assert expected result: HTTP 200 (allowed) or curl exit/status 0 (blocked).

The probe pod approach is simpler than ephemeral containers (used by netassert v2) and requires no extra cluster privileges. For HTTP-only testing in a learning tool, it is the correct choice.

### How External Ingress Testing Works

Tools like k6, the Gateway API conformance suite, and integration test frameworks make HTTP requests from the test process (not from a pod) to the Ingress/Gateway external IP. The workflow:

1. Resolve the external IP from `Ingress.status.loadBalancer.ingress[0].ip` or `Gateway.status.addresses[0].value`.
2. Set the `Host` header to the virtual hostname used in the HTTPRoute/Ingress rule.
3. Send request with `net/http` client.
4. Check the response status code.

In Kind with cloud-provider-kind, the external IP is a Docker network IP reachable from the host. No special tunneling needed. (MEDIUM confidence — cloud-provider-kind v0.9.0+ is documented to handle Ingress natively.)

### How TLS Validation Works in Go

The Go standard library provides everything needed:

- `tls.Dial` or `net/http` with custom `tls.Config` transport establishes the TLS connection.
- `conn.ConnectionState().PeerCertificates[0]` returns the leaf certificate.
- `cert.NotAfter` — expiry timestamp; compare with `time.Now()`.
- `cert.DNSNames` — SAN list; check if expected hostname is present.
- `tls.Config{InsecureSkipVerify: true}` — skips chain verification for self-signed certs; still retrieves PeerCertificates for manual SAN/expiry checks.

All three TLS checks (TLS-01/02/03) share a single `tls.Dial` call. The spec should include a `tls` block on `ConnectivityCheck` rather than separate validation entries.

---

## Sources

- [Kubernetes NetworkPolicy documentation](https://kubernetes.io/docs/concepts/services-networking/network-policies/)
- [Tufin/test-network-policies — probe pod pattern](https://github.com/Tufin/test-network-policies)
- [controlplaneio/netassert — ephemeral container pattern](https://github.com/controlplaneio/netassert)
- [Gateway API v1.4 release notes](https://kubernetes.io/blog/2025/11/06/gateway-api-v1-4/)
- [Gateway API Go module — sigs.k8s.io/gateway-api](https://pkg.go.dev/sigs.k8s.io/gateway-api)
- [Kind Ingress documentation — cloud-provider-kind](https://kind.sigs.k8s.io/docs/user/ingress/)
- [kubernetes-sigs/cloud-provider-kind](https://github.com/kubernetes-sigs/cloud-provider-kind)
- [Go crypto/tls package documentation](https://pkg.go.dev/crypto/tls)
- [cert-manager SelfSigned issuer documentation](https://cert-manager.io/docs/configuration/selfsigned/)
- [kubeasy-cli internal/validation/types.go and executor.go — existing ConnectivitySpec](../../../internal/validation/types.go)

---

*Feature research for: kubeasy-cli connectivity validation extension (v2.7.0)*
*Researched: 2026-03-11*
