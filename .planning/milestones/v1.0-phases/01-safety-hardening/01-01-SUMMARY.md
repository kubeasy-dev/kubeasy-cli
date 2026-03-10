---
phase: 01-safety-hardening
plan: "01"
subsystem: validation, cmd
tags: [tdd, testing, safety, red-phase]
dependency_graph:
  requires: []
  provides: [01-02, 01-03]
  affects: [internal/validation/executor.go, internal/validation/loader.go]
tech_stack:
  added: []
  patterns: [TDD red-phase, table-driven tests, require.NotPanics]
key_files:
  created:
    - cmd/common_test.go
  modified:
    - internal/validation/executor_test.go
    - internal/validation/loader_test.go
decisions:
  - "Used require.NotPanics() to capture bare type assertion panics as RED test failures"
  - "TestGetGVRForKind tests already pass — getGVRForKind already returns errors for unsupported kinds"
  - "TestFindLocalChallengeFile_NoHardcodedPath passes vacuously — verified via _HonorsEnvVar RED test"
metrics:
  duration: 4m
  completed: "2026-03-09"
  tasks_completed: 2
  files_changed: 3
---

# Phase 01 Plan 01: TDD Red Phase — Safety Behavior Tests Summary

**One-liner:** Failing tests for 5 safety behaviors (bare type assertion panics, hardcoded path) using require.NotPanics and env var assertions to lock RED state before production fixes.

## What Was Built

Three test additions covering the five Phase 1 safety behaviors:

### internal/validation/executor_test.go

Added `TestExecute_MalformedSpec` (6 subtests) and `TestGetGVRForKind_UnsupportedKind` + `TestGetGVRForKind_SupportedKinds`.

- `TestExecute_MalformedSpec` is RED: bare type assertions in executor.go panic on wrong Spec type. All 6 subtests (5 wrong-type + nil) fail because `require.NotPanics` catches the panics as test failures.
- `TestGetGVRForKind_UnsupportedKind` passes immediately — `getGVRForKind` already returns `"unsupported resource kind"` errors for CronJob, Namespace, ClusterRole.
- `TestGetGVRForKind_SupportedKinds` passes — all 7 supported kinds return valid GVRs.

### internal/validation/loader_test.go

Added `TestFindLocalChallengeFile_NoHardcodedPath` and `TestFindLocalChallengeFile_HonorsEnvVar`.

- `TestFindLocalChallengeFile_HonorsEnvVar` is RED: the env var `KUBEASY_LOCAL_CHALLENGES_DIR` lookup doesn't exist in loader.go yet, so FindLocalChallengeFile returns empty even when the env var points to a valid directory.
- `TestFindLocalChallengeFile_NoHardcodedPath` passes vacuously — no file exists at the hardcoded `~/Workspace/...` path on this machine.

### cmd/common_test.go (new file)

Added `TestValidateChallengeSlug` with 12 table-driven subtests covering valid slugs (pod-evicted, basic-pod, config-map-101, a1b) and invalid slugs (uppercase, spaces, too short, empty, leading/trailing hyphens). All pass — `validateChallengeSlug` already exists and works correctly.

## Test RED/GREEN State

| Test | State | Reason |
|------|-------|--------|
| TestExecute_MalformedSpec (6 subtests) | RED | Bare type assertions panic |
| TestGetGVRForKind_UnsupportedKind | PASS | Already returns error |
| TestGetGVRForKind_SupportedKinds | PASS | Already returns correct GVR |
| TestFindLocalChallengeFile_HonorsEnvVar | RED | Env var lookup missing from loader.go |
| TestFindLocalChallengeFile_NoHardcodedPath | PASS (vacuous) | No real file at hardcoded path |
| TestValidateChallengeSlug (12 subtests) | PASS | Function already correct |

## Verification

```
go build ./...                          # PASS — compiles cleanly
FAIL github.com/kubeasy-dev/kubeasy-cli/internal/validation  # Only failing package (expected)
ok   github.com/kubeasy-dev/kubeasy-cli/cmd                  # All pass
```

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check

Verified files exist:
- internal/validation/executor_test.go contains TestExecute_MalformedSpec: YES
- internal/validation/loader_test.go contains TestFindLocalChallengeFile_HonorsEnvVar: YES
- cmd/common_test.go contains TestValidateChallengeSlug: YES

Commits:
- 0ba4344: test(01-01): add failing tests for Execute() malformed spec and getGVRForKind
- d700d50: test(01-01): add failing tests for FindLocalChallengeFile env var and validateChallengeSlug

## Self-Check: PASSED
