---
gsd_state_version: 1.0
milestone: v2.7.0
milestone_name: "Connectivity Extension"
status: defining_requirements
stopped_at: ""
last_updated: "2026-03-11T00:00:00Z"
last_activity: "2026-03-11 — Milestone v2.7.0 started"
progress:
  total_phases: 0
  completed_phases: 0
  total_plans: 0
  completed_plans: 0
  percent: 0
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-11 after v2.7.0 milestone start)

**Core value:** The validation system must be robust, extensible, and test-covered — so that adding a new validation type is simple and safe.
**Current focus:** Defining requirements for v2.7.0

## Current Position

Phase: Not started (defining requirements)
Plan: —
Status: Defining requirements
Last activity: 2026-03-11 — Milestone v2.7.0 started

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- Brownfield — no architectural refactor; fix implementations only
- Tests first on critical commands to reduce regression risk in later refactors
- Comma-ok on all Spec assertions to return Result instead of panicking
- [v1.0]: buildCurlCommand returns a direct arg slice — no shell invoked
- [v1.0]: fetchManifestAllowedPrefixes validates URLs before http.Get
- [v2.7.0]: cloud-provider-kind preferred over extraPortMappings for LoadBalancer IPs in Kind — avoids privileged port binding (80/443) on host
- [v2.7.0]: External connectivity validation runs from CLI (net/http) not pod exec — cleaner for Ingress/Gateway API testing
- [v2.7.0]: Probe pod managed by CLI for internal connectivity when no sourcePod has curl — namespace configurable by challenge designer

### Pending Todos

None yet.

### Blockers/Concerns

None yet.
