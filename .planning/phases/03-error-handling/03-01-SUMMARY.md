---
phase: 03-error-handling
plan: 01
subsystem: kube
tags: [go, kubernetes, dynamic-client, error-handling, tdd, fake-client]

# Dependency graph
requires: []
provides:
  - ApplyManifest returns first critical error (create/update failures are fatal, not silently skipped)
  - Four targeted unit tests covering critical vs skippable error classification
affects: [deployer, challenge-start-command]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "fail-fast error propagation in manifest application loop"
    - "fake dynamic client with PrependReactor for error injection in unit tests"

key-files:
  created: []
  modified:
    - internal/kube/manifest.go
    - internal/kube/manifest_test.go

key-decisions:
  - "Critical errors (create forbidden/quota, update failure, get-for-update failure) return immediately with wrapped error"
  - "Skippable errors (decode failure, RESTMapping not found, IsNotFound on create) continue to next document"
  - "IsAlreadyExists on create triggers update path — consistent with pre-existing behavior"

patterns-established:
  - "Reactor injection pattern: fake.NewSimpleDynamicClient(scheme) + PrependReactor for error scenarios"
  - "Pre-populate fake client with Unstructured object to trigger AlreadyExists path"

requirements-completed: [ERR-01]

# Metrics
duration: 4min
completed: 2026-03-09
---

# Phase 3 Plan 01: ApplyManifest Critical Error Handling Summary

**ApplyManifest now returns the first critical error (forbidden create, update failure) instead of silently swallowing all errors and returning nil, enabling kubeasy challenge start to exit non-zero on broken manifests.**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-09T15:28:48Z
- **Completed:** 2026-03-09T15:32:14Z
- **Tasks:** 2 (TDD Red + Green)
- **Files modified:** 2

## Accomplishments

- Added four targeted tests proving the critical/skippable error classification
- Fixed three code paths in ApplyManifest that silently continued on critical failures
- Full unit test suite remains green (71 tests in kube package, 28 ApplyManifest-specific)
- Removed stale comment "Return nil even if some documents failed" from manifest.go

## Task Commits

Each task was committed atomically:

1. **Task 1: Create manifest_test.go with four ApplyManifest tests (RED)** - `c7784e5` (test)
2. **Task 2: Fix ApplyManifest to return first critical error (GREEN)** - `e44ef8d` (fix)

_Note: TDD tasks have two commits (test RED → fix GREEN)_

## Files Created/Modified

- `internal/kube/manifest.go` - Three error paths changed from log+continue to return wrapped error; stale comment removed
- `internal/kube/manifest_test.go` - Four new tests added: CreateFailure_Critical, UpdateFailure_Critical, DecodeError_Skipped, IsNotFound_Skipped

## Decisions Made

- Followed locked decisions from CONTEXT.md exactly: create/update failures are CRITICAL, decode/RESTMapping/IsNotFound are SKIPPABLE
- Used `fake.NewSimpleDynamicClient` with `PrependReactor` for error injection rather than interface mocks — consistent with existing test patterns in the file
- Pre-populated fake client with `Unstructured` object to trigger the AlreadyExists path for the update failure test

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

The Edit tool failed on manifest.go due to tab/space ambiguity in old_string matching. Used Python string replacement to handle exact tab-indented content — no functional impact.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- ApplyManifest now propagates errors to deployer.DeployChallenge (caller already handled non-nil returns)
- DeployChallenge propagates to challenge start command — kubeasy challenge start will now exit non-zero on manifest failures
- Ready for Plan 03-02 (next error handling improvement)

---
*Phase: 03-error-handling*
*Completed: 2026-03-09*
