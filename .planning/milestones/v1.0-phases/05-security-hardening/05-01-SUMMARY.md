---
phase: 05-security-hardening
plan: 01
subsystem: validation
tags: [security, shell-injection, curl, pod-exec, tdd]

# Dependency graph
requires: []
provides:
  - buildCurlCommand helper that passes URL as positional arg directly to PodExecOptions.Command
  - Rewritten checkConnectivity curl block with no shell invocation
  - escapeShellArg deleted (no callers)
  - wget fallback annotated with TODO(sec) comment
affects: [future-connectivity-validation, 05-security-hardening]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Direct exec arg slice pattern: pass URL as positional arg to curl, no sh -c wrapper"
    - "buildCurlCommand pure helper: pure function returning []string, testable without a cluster"

key-files:
  created: []
  modified:
    - internal/validation/executor.go
    - internal/validation/executor_test.go

key-decisions:
  - "buildCurlCommand returns a direct arg slice starting with 'curl' — no shell is ever invoked"
  - "escapeShellArg deleted entirely; no callers remain after curl block rewrite"
  - "wget fallback left as sh -c with TODO(sec) comment — deferred to future security phase"
  - "TestEscapeShellArg removed alongside deleted function — test had no value without its target"

patterns-established:
  - "Pod exec commands must pass args as direct slice elements, never via sh -c string interpolation"

requirements-completed:
  - SEC-01

# Metrics
duration: 3min
completed: 2026-03-11
---

# Phase 5 Plan 01: Security Hardening — Shell Injection Fix Summary

**Replaced `sh -c "curl ... '$URL'"` shell string interpolation with `buildCurlCommand` direct-arg slice in executeConnectivity, eliminating the curl shell injection vector entirely**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-11T08:11:53Z
- **Completed:** 2026-03-11T08:14:53Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments

- Added `buildCurlCommand(url string, timeoutSeconds int) []string` pure helper that returns a direct curl arg slice — no shell invoked
- Rewrote `checkConnectivity` curl block to call `buildCurlCommand(target.URL, timeout)` directly
- Deleted `escapeShellArg` function (no remaining callers) and its test `TestEscapeShellArg`
- Updated wget fallback to reference `target.URL` directly and added `TODO(sec)` comment
- Added 5 `TestCheckConnectivity_*` tests verifying the direct-arg contract (no `sh`/`-c`, URL verbatim at last position, timeout value correct)
- Full unit suite passes (no regressions)

## Task Commits

Each task was committed atomically:

1. **Task 1: buildCurlCommand tests (RED+GREEN) + fix curl + delete escapeShellArg** - `6d582cc` (fix)
2. **Task 2: Full unit suite passes** - verified in Task 1 commit (no additional files)

**Plan metadata:** TBD (docs commit)

## Files Created/Modified

- `internal/validation/executor.go` - Added `buildCurlCommand` helper, rewrote curl block, deleted `escapeShellArg`, fixed wget fallback
- `internal/validation/executor_test.go` - Removed `TestEscapeShellArg`, added 5 `TestCheckConnectivity_*` tests

## Decisions Made

- `buildCurlCommand` returns a direct `[]string` — no shell is invoked at any point in the curl path
- `escapeShellArg` deleted entirely with its test — keeping it would imply it's still useful somewhere
- wget fallback left as `sh -c` with `TODO(sec)` — wget argument semantics differ from curl; fixing it is out of scope for this plan

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed TestEscapeShellArg referencing deleted function**
- **Found during:** Task 1 (GREEN phase — running tests after deleting escapeShellArg)
- **Issue:** `executor_test.go` line 2040 called `escapeShellArg(tt.input)` which no longer exists; build failed with "undefined: escapeShellArg"
- **Fix:** Removed the `TestEscapeShellArg` test block entirely (the deleted function has no test value)
- **Files modified:** `internal/validation/executor_test.go`
- **Verification:** Build succeeded after removal; all 5 new tests pass
- **Committed in:** `6d582cc` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 — bug/broken build caused by deleting function that had a test)
**Impact on plan:** Required fix; the test was testing behavior of the now-deleted function. No scope creep.

## Issues Encountered

None beyond the pre-existing `TestEscapeShellArg` test that needed removal alongside its target function.

## Next Phase Readiness

- Shell injection via curl path in `executeConnectivity` is eliminated
- wget fallback still uses `sh -c` — `TODO(sec)` comment marks it for a future security phase
- All unit tests pass; lint is clean

---
*Phase: 05-security-hardening*
*Completed: 2026-03-11*

## Self-Check: PASSED

- executor.go: FOUND
- executor_test.go: FOUND
- 05-01-SUMMARY.md: FOUND
- commit 6d582cc: FOUND
