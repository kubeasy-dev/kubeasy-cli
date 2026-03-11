---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: completed
stopped_at: Completed 07-probe-pod-internal-connectivity-02-PLAN.md
last_updated: "2026-03-11T11:30:47.945Z"
last_activity: 2026-03-11 — Phase 7 Plan 02 complete; probe wiring + connectivity fixes; 297 tests pass
progress:
  total_phases: 4
  completed_phases: 2
  total_plans: 6
  completed_plans: 6
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-11 after v2.7.0 milestone start)

**Core value:** The validation system must be robust, extensible, and test-covered — so that adding a new validation type is simple and safe.
**Current focus:** Phase 7 — Probe Pod Internal Connectivity (complete)

## Current Position

Phase: 7 of 9 (Probe Pod Internal Connectivity)
Plan: 02 of 02 (complete)
Status: Phase complete — ready to begin Phase 8
Last activity: 2026-03-11 — Phase 7 Plan 02 complete; probe wiring + connectivity fixes; 297 tests pass

Progress: [░░░░░░░░░░] 0%

## Performance Metrics

**Velocity:**
- Total plans completed: 14 (v1.0)
- Average duration: —
- Total execution time: —

**By Phase (v2.7.0):**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| — | — | — | — |
| Phase 06-infrastructure-foundation P01 | 4min | 2 tasks | 4 files |
| Phase 06-infrastructure-foundation P03 | 7min | 2 tasks | 2 files |
| Phase 06-infrastructure-foundation P02 | 12min | 2 tasks | 4 files |
| Phase 06-infrastructure-foundation P04 | 25 | 3 tasks | 5 files |
| Phase 07-probe-pod-internal-connectivity P01 | 201s | 3 tasks | 4 files |
| Phase 07-probe-pod-internal-connectivity P02 | 24min | 3 tasks | 4 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [v1.0]: buildCurlCommand returns a direct arg slice — no shell invoked
- [v1.0]: fetchManifestAllowedPrefixes validates URLs before http.Get
- [v2.7.0]: cloud-provider-kind preferred for LoadBalancer IPs; not auto-installed (host daemon requiring sudo)
- [v2.7.0]: External connectivity runs from CLI host via net/http — not pod exec
- [v2.7.0]: Probe pod lifecycle lives in deployer/, not validation/ — executor stays cluster-read-only
- [v2.7.0]: connectivity `mode` field discriminant (internal/external) — no new ValidationType to preserve backend compat
- [v2.7.0]: Gateway API CRDs pinned to v1.2.1 (not v1.5.0) — v1.5.0 requires server-side apply
- [Phase 06-01]: Path constants implemented as functions not vars: os.UserHomeDir() called at runtime for portability
- [Phase 06-01]: File permissions 0o600 for kind-config.yaml per gosec G306 security requirements
- [Phase 06-01]: nolint:unused directive for Wave 1 helpers (writeKindConfig, hasExtraPortMappings) used by plan 04 setup.go
- [Phase 06-03]: installCertManager uses *kubernetes.Clientset (not Interface) to satisfy kube.WaitForDeploymentsReady signature
- [Phase 06-03]: waitForCertManagerWebhookEndpoints uses legacy corev1.Endpoints API — matches cert-manager webhook service registration
- [Phase 06-02]: Discovery().ServerResourcesForGroupVersion() used for Gateway API CRD check — avoids apiextensions-apiserver import
- [Phase 06-02]: cloudProviderKindBinaryURLForPlatform(goos, goarch) extracted as testable variant — cloudProviderKindBinaryURL() delegates to it
- [Phase 06-02]: downloadCloudProviderKind uses net/http directly — kube.FetchManifest URL allowlist rejects binary download URLs
- [Phase 06-04]: SetupAllComponents accepts *kubernetes.Clientset (not Interface) — satisfies installCertManager and WaitForDeploymentsReady concrete-type requirement
- [Phase 06-04]: printComponentResult lives in cmd/setup.go not deployer/ — UI concern, not deployment concern
- [Phase 06-04]: SetupInfrastructure kept (not deleted) — only active callers migrated to eliminate SA1019 deprecation lint
- [Phase 07-probe-pod-internal-connectivity]: DeleteProbePod uses context.Background()+10s internally (not caller ctx) to guarantee cleanup on cancellation (PROBE-03)
- [Phase 07-02]: restConfig.Host emptiness used as test-environment guard for fake clientset (non-nil RESTClient with nil internal client panics on Post())
- [Phase 07-02]: blocked-as-expected per-target message not propagated to overall result (only failures are added to messages); tests assert result.Passed==true without message check
- [Phase 07-02]: validateSourcePod is a no-op (probe mode relaxation) — empty sourcePod is valid

### Pending Todos

None yet.

### Blockers/Concerns

- [Phase 6]: Kind cluster extraPortMappings (INFRA-06) cannot be patched on existing clusters — audit setup.go for `--reset` flag; may require cluster recreation for existing users
- [Phase 6]: cert-manager webhook needs 15–30 s post-Ready polling on Endpoints object, not just ReadyReplicas
- [Phase 6]: INFRA-02/03 require two-pass REST mapper refresh: apply CRDs, rebuild mapper, then apply GatewayClass resources
- [Phase 7]: Probe pod concurrency model unresolved — single shared pod vs per-key pods; decide before writing plan
- [Phase 8]: macOS Docker IP reachability with cloud-provider-kind v0.10.0 is MEDIUM confidence — verify locally before finalizing EXT-03 NodePort fallback

## Session Continuity

Last session: 2026-03-11T11:25:00Z
Stopped at: Completed 07-probe-pod-internal-connectivity-02-PLAN.md
Resume file: None
