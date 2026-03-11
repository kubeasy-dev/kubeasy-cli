---
gsd_state_version: 1.0
milestone: v2.7.0
milestone_name: "Connectivity Extension"
status: ready_to_plan
stopped_at: ""
last_updated: "2026-03-11T00:00:00Z"
last_activity: "2026-03-11 — Roadmap created, 4 phases defined (6–9), 20/20 requirements mapped"
progress:
  total_phases: 4
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-11 after v2.7.0 milestone start)

**Core value:** The validation system must be robust, extensible, and test-covered — so that adding a new validation type is simple and safe.
**Current focus:** Phase 6 — Infrastructure Foundation (ready to plan)

## Current Position

Phase: 6 of 9 (Infrastructure Foundation)
Plan: — (not started)
Status: Ready to plan
Last activity: 2026-03-11 — Roadmap created, phases 6–9 defined, 20/20 v2.7.0 requirements mapped

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

### Pending Todos

None yet.

### Blockers/Concerns

- [Phase 6]: Kind cluster extraPortMappings (INFRA-06) cannot be patched on existing clusters — audit setup.go for `--reset` flag; may require cluster recreation for existing users
- [Phase 6]: cert-manager webhook needs 15–30 s post-Ready polling on Endpoints object, not just ReadyReplicas
- [Phase 6]: INFRA-02/03 require two-pass REST mapper refresh: apply CRDs, rebuild mapper, then apply GatewayClass resources
- [Phase 7]: Probe pod concurrency model unresolved — single shared pod vs per-key pods; decide before writing plan
- [Phase 8]: macOS Docker IP reachability with cloud-provider-kind v0.10.0 is MEDIUM confidence — verify locally before finalizing EXT-03 NodePort fallback

## Session Continuity

Last session: 2026-03-11
Stopped at: Roadmap created — ready to begin planning Phase 6
Resume file: None
