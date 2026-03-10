---
phase: 02-command-test-coverage
plan: 01
subsystem: cmd/start
tags: [tdd, unit-tests, function-vars, injection]
dependency_graph:
  requires: []
  provides: [start-cmd-unit-tests, function-var-injection-pattern]
  affects: [cmd/start.go, cmd/start_test.go, cmd/main_test.go]
tech_stack:
  added: []
  patterns: [function-var injection for testability, TestMain CI mode setup]
key_files:
  created:
    - cmd/start_test.go
    - cmd/main_test.go
  modified:
    - cmd/start.go
decisions:
  - "Used ui.SetCIMode(true) in TestMain to suppress pterm spinner goroutine data races under -race"
  - "Function vars (apiGetChallenge, apiGetChallengeProgress, apiStartChallenge) front direct api.* calls to enable test injection"
metrics:
  duration: 2m
  completed: 2026-03-09
  tasks: 1
  files: 3
---

# Phase 2 Plan 1: Start Command RunE Guard Tests Summary

**One-liner:** Four unit tests for start.go RunE guards via function-var injection with CI mode to eliminate pterm race conditions.

## What Was Built

Added three package-level function variables to `cmd/start.go` fronting the `api.GetChallenge`, `api.GetChallengeProgress`, and `api.StartChallenge` calls. These vars allow tests to inject stubs without mocking entire packages.

Created `cmd/start_test.go` with four tests covering the pre-Kubernetes guard paths in RunE:
- `TestStartRunE_InvalidSlug` — uppercase slug rejected before any API call
- `TestStartRunE_AlreadyInProgress` — returns nil when progress status is "in_progress"
- `TestStartRunE_AlreadyCompleted` — returns nil when progress status is "completed"
- `TestStartRunE_APIFailure` — returns non-nil error when GetChallenge fails, no panic

Created `cmd/main_test.go` with `TestMain` that enables `ui.SetCIMode(true)` before running all cmd tests.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] pterm spinner data race under -race detector**

- **Found during:** GREEN phase — tests passed functionally but `-race` detector flagged pterm's `SpinnerPrinter` internal goroutine
- **Issue:** `ui.WaitMessage` starts a pterm spinner goroutine; when the spinner is stopped from the test goroutine, the race detector flags concurrent reads/writes in pterm's internal state
- **Fix:** Added `cmd/main_test.go` with `TestMain` that calls `ui.SetCIMode(true)` — CI mode bypasses the spinner entirely, using plain text output instead
- **Files modified:** `cmd/main_test.go` (created)
- **Commit:** 22301bc

## Decisions Made

| Decision | Rationale |
|----------|-----------|
| TestMain with SetCIMode(true) instead of per-test setup | Package-level setup ensures all future cmd tests also avoid the pterm race by default |
| Function vars over interface injection | Matches existing codebase pattern; minimal diff to start.go; vars are unexported so tests in same package access them directly |

## Self-Check

Files created/modified:
- `cmd/start_test.go` — exists
- `cmd/main_test.go` — exists
- `cmd/start.go` — modified with three function vars

Commits:
- 22301bc — test(02-01): add function vars to start.go and unit tests for RunE guards

## Self-Check: PASSED
