---
phase: 08-external-http
plan: 01
subsystem: validation
tags: [connectivity, external-http, yaml, types, loader]

# Dependency graph
requires:
  - phase: 07-probe-pod-internal-connectivity
    provides: "ConnectivitySpec, SourcePod, ConnectivityCheck types; loader.go parseSpec TypeConnectivity case; validateSourcePod no-op"
provides:
  - "Mode string field on ConnectivitySpec discriminating internal vs external execution"
  - "HostHeader string field on ConnectivityCheck for Host header override on external requests"
  - "Parse-time validation: mode:external + sourcePod rejected with clear error"
  - "Parse-time validation: unknown mode values rejected with 'invalid mode' error"
  - "Full backwards compatibility: existing specs with no mode field parse unchanged"
  - "5 new loader tests covering all EXT-01/02/03 behaviors"
affects:
  - "08-02 executor — reads Mode field to dispatch internal vs external execution path"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Mode discriminant string field with omitempty for backwards-compatible extension of ConnectivitySpec"
    - "Parse-time validation of field combinations (mode:external + sourcePod = error) in loader.go parseSpec"

key-files:
  created:
    - internal/validation/loader_test.go (5 new test functions appended)
  modified:
    - internal/validation/types.go
    - internal/validation/loader.go

key-decisions:
  - "Mode field is empty string (not 'internal') by default — no migration needed for existing challenges"
  - "HostHeader field lives on ConnectivityCheck (per-target granularity), not ConnectivitySpec (spec-wide)"
  - "mode:external + sourcePod rejected at parse time not execution time — fail fast principle"
  - "Unknown mode values rejected with fmt.Errorf containing 'invalid mode' for testability"

patterns-established:
  - "TDD RED→GREEN: write failing tests first, confirm failure, then implement"
  - "Parse-time validation: prefer early rejection of incoherent specs over silent defaults"

requirements-completed: [EXT-01, EXT-02, EXT-03]

# Metrics
duration: 8min
completed: 2026-03-11
---

# Phase 8 Plan 01: External HTTP Type Contract Summary

**Mode and HostHeader fields added to connectivity types with parse-time validation that rejects mode:external+sourcePod combos and unknown modes while preserving full backwards compatibility**

## Performance

- **Duration:** 8 min
- **Started:** 2026-03-11T12:10:30Z
- **Completed:** 2026-03-11T12:17:30Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Added `Mode string` field to `ConnectivitySpec` with `omitempty` YAML/JSON tags — empty string means internal (no migration needed)
- Added `HostHeader string` field to `ConnectivityCheck` with `omitempty` — per-target Host header override for external mode
- Loader.go TypeConnectivity case now validates mode at parse time: rejects external+sourcePod, rejects unknown modes
- 5 new loader tests covering all required behaviors (ExternalMode, ExternalModeWithSourcePod, SslipIO, InvalidMode, NoMode)
- All 902 unit tests pass; no regressions

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Mode and HostHeader fields to types.go** - `1d088bd` (feat)
2. **Task 2: Loader parse-time validation + RED→GREEN tests** - `d6b1078` (feat)

**Plan metadata:** (docs commit follows)

_Note: TDD tasks had RED→GREEN flow — tests written first in loader_test.go, then loader.go updated_

## Files Created/Modified
- `internal/validation/types.go` - Added Mode field to ConnectivitySpec; HostHeader field to ConnectivityCheck
- `internal/validation/loader.go` - TypeConnectivity parseSpec case now validates Mode field
- `internal/validation/loader_test.go` - 5 new test functions for EXT-01/02/03 behaviors appended

## Decisions Made
- Mode field is empty string (not "internal") by default — no migration needed for existing challenges that omit the field
- HostHeader field lives on ConnectivityCheck (per-target) not ConnectivitySpec (spec-wide) — allows mixing Host-overridden and normal targets in one spec
- Parse-time rejection of mode:external+sourcePod is fail-fast — executor in Plan 02 never sees incoherent specs
- Unknown mode values get `fmt.Errorf("invalid mode %q: ...")` format for precise error messages

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Types and loader contract complete; Plan 02 executor can now dispatch on `spec.Mode == "external"` vs `""`/"internal"
- No blockers for Plan 02

## Self-Check: PASSED

- FOUND: internal/validation/types.go
- FOUND: internal/validation/loader.go
- FOUND: internal/validation/loader_test.go
- FOUND: .planning/phases/08-external-http/08-01-SUMMARY.md
- FOUND: commit 1d088bd (feat: types.go fields)
- FOUND: commit d6b1078 (feat: loader validation + tests)

---
*Phase: 08-external-http*
*Completed: 2026-03-11*
