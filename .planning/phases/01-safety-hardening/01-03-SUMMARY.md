---
phase: 01-safety-hardening
plan: "03"
subsystem: validation/cmd
tags: [safety, slug-validation, env-var, hardening]
dependency_graph:
  requires: ["01-01"]
  provides: ["SAFE-02", "SAFE-03", "TST-05"]
  affects: [internal/validation/loader.go, cmd/start.go, cmd/submit.go, cmd/clean.go]
tech_stack:
  added: []
  patterns: [env-var-override, fail-fast-validation]
key_files:
  created: []
  modified:
    - internal/validation/loader.go
    - cmd/start.go
    - cmd/submit.go
    - cmd/clean.go
decisions:
  - "Use env var override (KUBEASY_LOCAL_CHALLENGES_DIR) rather than hardcoded path; production binaries never check developer directories"
  - "Slug validation inserted as first statement in RunE before any API or cluster call for fail-fast behavior"
metrics:
  duration: 5m
  completed: 2026-03-09
---

# Phase 01 Plan 03: Remove Hardcoded Path and Add Slug Validation Summary

**One-liner:** Replaced hardcoded `~/Workspace/kubeasy/challenges/` dev path with `KUBEASY_LOCAL_CHALLENGES_DIR` env var override, and added `validateChallengeSlug` guards to `start`, `submit`, and `clean` commands before any API or cluster call.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Replace hardcoded path in FindLocalChallengeFile | 46044b8 | internal/validation/loader.go |
| 2 | Add validateChallengeSlug to start.go, submit.go, clean.go | e8b40af | cmd/start.go, cmd/submit.go, cmd/clean.go |

## Decisions Made

- **KUBEASY_LOCAL_CHALLENGES_DIR over hardcoded path:** The hardcoded `~/Workspace/kubeasy/challenges/` path was a developer-only convenience that could silently load stale local files in production. Replacing it with an explicit env var makes the override intentional and invisible when unset.
- **Fail-fast slug validation:** Placing `validateChallengeSlug` as the first statement in `RunE` ensures invalid slugs are rejected immediately with a clear error message, before any network or cluster operation is attempted.

## Verification

- `go build ./...` — passes
- `go test ./internal/validation/... -run TestFindLocalChallengeFile` — 5/5 PASS
  - `TestFindLocalChallengeFile_HonorsEnvVar` turned GREEN
  - `TestFindLocalChallengeFile_NoHardcodedPath` PASS
  - `TestFindLocalChallengeFile` (existing) PASS
- `go test ./cmd/... -run TestValidateChallengeSlug` — 13/13 PASS
- `task test:unit` — all PASS

## Deviations from Plan

None - plan executed exactly as written.

## Self-Check

### Files exist

- [x] `internal/validation/loader.go` — modified
- [x] `cmd/start.go` — modified
- [x] `cmd/submit.go` — modified
- [x] `cmd/clean.go` — modified

### Commits exist

- [x] 46044b8 — fix(validation): replace hardcoded dev path with KUBEASY_LOCAL_CHALLENGES_DIR env var
- [x] e8b40af — feat(cmd): add slug validation before API/cluster calls in start, submit, clean

## Self-Check: PASSED
