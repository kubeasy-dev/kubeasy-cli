---
phase: 04-code-quality
plan: 01
subsystem: api
tags: [refactor, cleanup, api, backward-compat]
dependency_graph:
  requires: []
  provides: [canonical-api-surface]
  affects: [cmd/start.go, cmd/submit.go, cmd/reset.go, cmd/common.go]
tech_stack:
  added: []
  patterns: [direct-canonical-calls, no-alias-wrappers]
key_files:
  modified:
    - internal/api/client.go
    - internal/api/client_http_test.go
    - cmd/common.go
    - cmd/start.go
    - cmd/submit.go
    - cmd/reset.go
decisions:
  - Alias functions deleted entirely rather than deprecated — no grace period needed since all callers are in the same repo
  - cmd/submit.go: inline SubmitChallenge replaces SendSubmit; allPassed branch now also checks submitResult.Success
  - cmd/reset.go: inline ResetChallenge replaces ResetChallengeProgress; success check surfaced to caller
  - client_http_test.go: alias-specific tests replaced with canonical function tests covering the same behavior
metrics:
  duration: 6m
  completed_date: "2026-03-10"
  tasks_completed: 2
  files_modified: 6
---

# Phase 4 Plan 01: Remove API Alias Functions Summary

**One-liner:** Deleted six backward-compat wrapper functions from api/client.go and updated all cmd/ callers to use canonical names (GetChallengeBySlug, GetChallengeStatus, StartChallengeWithResponse, SubmitChallenge, ResetChallenge).

## What Was Done

### Task 1: Delete alias functions from internal/api/client.go

Removed six wrapper functions that created a double layer of indirection:

| Deleted alias | Canonical replacement |
|---|---|
| `GetUserProfile` | `GetProfile` |
| `GetChallenge` | `GetChallengeBySlug` |
| `GetChallengeProgress` | `GetChallengeStatus` |
| `StartChallenge` | `StartChallengeWithResponse` |
| `SendSubmit` | `SubmitChallenge` |
| `ResetChallengeProgress` | `ResetChallenge` |

Updated `client_http_test.go` to replace alias-specific tests with canonical function tests.

### Task 2: Update cmd/ callers

| File | Change |
|---|---|
| `cmd/common.go` | `api.GetChallenge` → `api.GetChallengeBySlug` |
| `cmd/start.go` | var block: all three vars updated to canonical names; `apiStartChallenge` type updated to return `(*ChallengeStartResponse, error)`; call site discards response with `_` |
| `cmd/submit.go` | var block updated; `api.SendSubmit` calls replaced with inline `api.SubmitChallenge` + `submitResult.Success` check |
| `cmd/reset.go` | `api.ResetChallengeProgress` replaced with `api.ResetChallenge` + inline success check |

## Verification

```
go build ./...          → success (all packages compile)
task test:unit          → 9 packages, all PASS
grep alias names        → zero matches in any .go file
```

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Test file referenced deleted alias functions**
- **Found during:** Task 1 commit (pre-commit hook typecheck)
- **Issue:** `client_http_test.go` had 8 tests calling deleted functions (GetUserProfile, StartChallenge, SendSubmit, ResetChallengeProgress, GetChallenge, GetChallengeProgress)
- **Fix:** Replaced alias-specific tests with canonical function equivalents testing the same behavior
- **Files modified:** `internal/api/client_http_test.go`
- **Commit:** df51429 (combined with Task 2 due to pre-commit hook requiring full compilation)

**Note on commit structure:** Tasks 1 and 2 were combined into a single commit because the pre-commit hook runs `golangci-lint` against the full project. Committing Task 1 in isolation (with cmd/ callers still referencing deleted functions) would always fail the hook. The combined commit is the only viable atomic unit that passes all checks.

## Self-Check: PASSED

- SUMMARY.md exists at `.planning/phases/04-code-quality/04-01-SUMMARY.md`
- Commit df51429 exists in git log
- All 6 key files modified (verified via git show)
- Zero alias function references remain in any .go file
