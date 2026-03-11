# Roadmap: kubeasy-cli

## Milestones

- ✅ **v1.0 Réduction de la dette technique** — Phases 1–5 (shipped 2026-03-11)
- 🚧 **v2.7.0 Connectivity Extension** — Phases 6–9 (in progress)

## Phases

<details>
<summary>✅ v1.0 Réduction de la dette technique (Phases 1–5) — SHIPPED 2026-03-11</summary>

- [x] Phase 1: Safety Hardening (3/3 plans) — completed 2026-03-09
- [x] Phase 2: Command Test Coverage (3/3 plans) — completed 2026-03-09
- [x] Phase 3: Error Handling (3/3 plans) — completed 2026-03-09
- [x] Phase 4: Code Quality (3/3 plans) — completed 2026-03-10
- [x] Phase 5: Security Hardening (2/2 plans) — completed 2026-03-10

Full details: `.planning/milestones/v1.0-ROADMAP.md`

</details>

### 🚧 v2.7.0 Connectivity Extension (In Progress)

**Milestone Goal:** Extend connectivity validation to support NetworkPolicy testing (probe pod), external Ingress/Gateway API validation (CLI HTTP client), and TLS certificate inspection — with the matching cluster infrastructure (nginx-ingress, Gateway API CRDs, cert-manager, cloud-provider-kind advisory).

- [x] **Phase 6: Infrastructure Foundation** — Install nginx-ingress, Gateway API CRDs + controller, cert-manager; Kind cluster extraPortMappings; cloud-provider-kind detection; setup status reporting (completed 2026-03-11)
- [x] **Phase 7: Probe Pod + Internal Connectivity** — Auto-managed probe pod for NetworkPolicy testing; cross-namespace source; blocked-connection (status 0) assertion; wget fallback removal (completed 2026-03-11)
- [x] **Phase 8: External HTTP** — CLI-side HTTP requests for Ingress/Gateway API validation; Host header; IP auto-resolution from Ingress/Gateway resources; macOS fallback (completed 2026-03-11)
- [ ] **Phase 9: TLS Validation** — Certificate expiry, SAN hostname matching, insecureSkipVerify for self-signed certs in Kind

## Phase Details

### Phase 6: Infrastructure Foundation
**Goal**: Users can run `kubeasy setup` and get nginx-ingress, Gateway API, and cert-manager installed and verified — with clear feedback on each component's readiness and an actionable message if cloud-provider-kind is missing
**Depends on**: Phase 5 (v1.0)
**Requirements**: INFRA-01, INFRA-02, INFRA-03, INFRA-04, INFRA-05, INFRA-06, INFRA-07
**Success Criteria** (what must be TRUE):
  1. User runs `kubeasy setup` and nginx-ingress controller is deployed and ready in the cluster
  2. User runs `kubeasy setup` and Gateway API v1 CRDs are installed; a GatewayClass backed by cloud-provider-kind is registered
  3. User runs `kubeasy setup` and cert-manager is deployed and its webhook is ready to accept certificate resources
  4. User sees a named status line per new component (ready / not-ready / missing) in `kubeasy setup` output
  5. User sees a clear advisory message with install instructions if cloud-provider-kind is not detected; setup does not fail
**Plans**: 4 plans

Plans:
- [ ] 06-01-PLAN.md — Foundation: ComponentResult type, version/path constants, Kind config I/O
- [ ] 06-02-PLAN.md — nginx-ingress + Gateway API + cloud-provider-kind installers
- [ ] 06-03-PLAN.md — cert-manager installer (two-pass apply + webhook polling)
- [ ] 06-04-PLAN.md — setup.go wiring: retrofit existing components + per-component status output

### Phase 7: Probe Pod + Internal Connectivity
**Goal**: Users can write connectivity validations without a `sourcePod` — the CLI auto-deploys and cleans up a curl probe pod — and can assert that a connection is blocked (expected status 0) across namespaces
**Depends on**: Phase 6
**Requirements**: PROBE-01, PROBE-02, PROBE-03, PROBE-04, CONN-01, CONN-02
**Success Criteria** (what must be TRUE):
  1. A connectivity validation with no `sourcePod` executes successfully using a CLI-managed probe pod that appears and disappears around the validation run
  2. The probe pod is removed even when the user interrupts the CLI with Ctrl-C (cleanup uses an independent context)
  3. Challenge spec can target a probe pod in a different namespace than the challenge namespace via `probeNamespace`
  4. A connectivity validation with `expectedStatus: 0` passes when the connection is blocked (timeout or refused)
  5. Source pod namespace is configurable via `sourceNamespace` enabling cross-namespace NetworkPolicy test scenarios
**Plans**: 2 plans

Plans:
- [ ] 07-01-PLAN.md — Probe pod deployer: constants, CreateProbePod/DeleteProbePod/WaitForProbePodReady, SourcePod.Namespace field
- [ ] 07-02-PLAN.md — Executor wiring: probe lifecycle in executeConnectivity, status-0 guard, wget removal, namespace resolution, loader relaxation

### Phase 8: External HTTP
**Goal**: Users can validate external HTTP connectivity to Ingress or Gateway API resources — the CLI makes the request directly, resolves the IP from the resource status, and handles the macOS Docker routing gap
**Depends on**: Phase 6
**Requirements**: EXT-01, EXT-02, EXT-03, EXT-04
**Success Criteria** (what must be TRUE):
  1. A connectivity validation with `mode: external` sends an HTTP request from the CLI host (not via pod exec) and reports pass/fail
  2. External check with a `hostHeader` field routes correctly to the target Ingress/Gateway virtual host
  3. Challenge spec using a sslip.io hostname (e.g., `myapp.127-0-0-1.sslip.io:8080`) resolves correctly and routes to the Ingress/Gateway virtual host without any local DNS configuration
  4. External check works on macOS via sslip.io hostnames encoding localhost (127.0.0.1) — no Docker IP routing issues
**Plans**: 2 plans

Plans:
- [ ] 08-01-PLAN.md — Types + loader validation: Mode/HostHeader fields, parse-time mode validation, loader tests (RED→GREEN)
- [ ] 08-02-PLAN.md — Executor external mode: checkExternalConnectivity via net/http, executeConnectivity branch, executor tests (RED→GREEN)

### Phase 9: TLS Validation
**Goal**: Users can validate TLS certificates as part of external connectivity checks — expiry, hostname SANs, and self-signed cert tolerance are all controllable per validation spec
**Depends on**: Phase 8
**Requirements**: TLS-01, TLS-02, TLS-03
**Success Criteria** (what must be TRUE):
  1. An external check fails with a descriptive message when the server certificate is expired
  2. An external check fails with a descriptive message when the server hostname does not match the certificate SANs
  3. An external check with `insecureSkipVerify: true` succeeds against a self-signed cert issued by cert-manager in the Kind cluster
**Plans**: 2 plans

Plans:
- [ ] 09-01-PLAN.md — TLSConfig type + ConnectivityCheck.TLS field; loader YAML parsing tests (RED→GREEN)
- [ ] 09-02-PLAN.md — Executor TLS logic: cert probe via tls.Dialer, expiry/SAN checks, insecureSkipVerify transport (RED→GREEN)

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Safety Hardening | v1.0 | 3/3 | Complete | 2026-03-09 |
| 2. Command Test Coverage | v1.0 | 3/3 | Complete | 2026-03-09 |
| 3. Error Handling | v1.0 | 3/3 | Complete | 2026-03-09 |
| 4. Code Quality | v1.0 | 3/3 | Complete | 2026-03-10 |
| 5. Security Hardening | v1.0 | 2/2 | Complete | 2026-03-10 |
| 6. Infrastructure Foundation | 4/4 | Complete   | 2026-03-11 | - |
| 7. Probe Pod + Internal Connectivity | 2/2 | Complete   | 2026-03-11 | - |
| 8. External HTTP | 2/2 | Complete   | 2026-03-11 | - |
| 9. TLS Validation | v2.7.0 | 0/2 | Not started | - |
