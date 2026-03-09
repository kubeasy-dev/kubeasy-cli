---
phase: 02-command-test-coverage
plan: 02
subsystem: testing
tags: [go, cobra, testify, function-vars, unit-tests, tdd]

# Dependency graph
requires:
  - phase: 01-safety-hardening
    provides: validateChallengeSlug helper used by submit.go RunE
provides:
  - submit.go with apiGetChallengeForSubmit and apiGetProgressForSubmit injectable vars
  - Four TestSubmitRunE_* unit tests covering all RunE guard paths
affects: [future command refactors, submit.go, testing patterns]

# Tech tracking
tech-stack:
  added: []
  patterns: [function-var injection for testable cobra commands]

key-files:
  created:
    - cmd/submit_test.go
  modified:
    - cmd/submit.go

key-decisions:
  - "Named vars apiGetChallengeForSubmit / apiGetProgressForSubmit (not apiGetChallenge / apiGetChallengeProgress) to avoid collision with start.go vars already established in 02-01"
  - "TDD RED first: wrote submit_test.go before adding vars to submit.go — confirmed compile-error failure before making green"

patterns-established:
  - "Pattern: Each command has its own distinctly-named function vars so parallel-wave plans never conflict"

requirements-completed: [TST-02]

# Metrics
duration: 5min
completed: 2026-03-09
---

# Phase 02 Plan 02: Submit Command RunE Guard Tests Summary

**Function-var injection on submit.go with four TDD-driven unit tests covering slug validation, nil progress, completed progress, and API failure guards**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-09T14:30:00Z
- **Completed:** 2026-03-09T14:35:00Z
- **Tasks:** 1 (TDD: RED + GREEN)
- **Files modified:** 2

## Accomplishments

- Added `apiGetChallengeForSubmit` and `apiGetProgressForSubmit` package-level vars to submit.go with injected defaults pointing to `api.GetChallenge` and `api.GetChallengeProgress`
- Replaced both direct api.* call sites in RunE with the injectable vars
- Wrote `TestSubmitRunE_InvalidSlug`, `TestSubmitRunE_ProgressNil`, `TestSubmitRunE_AlreadyCompleted`, `TestSubmitRunE_APIFailure` — all four pass with `-race`
- Full `task test:unit` suite remains green (45.7% statement coverage)

## Task Commits

1. **Task 1: Add function vars to submit.go and write RunE guard tests** - `9a249c8` (test)

**Plan metadata:** (pending docs commit)

## Files Created/Modified

- `cmd/submit.go` - Added two package-level function vars; replaced two api.* call sites
- `cmd/submit_test.go` - Four TestSubmitRunE_* unit tests (TDD-driven, package cmd)

## Decisions Made

- Used `apiGetChallengeForSubmit` and `apiGetProgressForSubmit` as names instead of reusing `apiGetChallenge` / `apiGetChallengeProgress` to prevent shadowing the vars already declared in start.go (both files share `package cmd`)
- TDD RED phase confirmed via compile-error failure before GREEN implementation

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- TST-02 satisfied: submit.go RunE guards are now covered by unit tests
- Pattern established for naming function vars per-command to avoid intra-package collisions
- Ready to continue with remaining command test plans in phase 02

## Self-Check: PASSED

- cmd/submit.go: FOUND
- cmd/submit_test.go: FOUND
- 02-02-SUMMARY.md: FOUND
- Commit 9a249c8: FOUND

---
*Phase: 02-command-test-coverage*
*Completed: 2026-03-09*
