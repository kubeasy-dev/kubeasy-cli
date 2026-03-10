---
phase: 05-security-hardening
plan: "02"
subsystem: internal/kube
tags: [security, allowlist, url-validation, tdd]
dependency_graph:
  requires: []
  provides: [fetchManifestAllowedPrefixes, FetchManifest-allowlist-guard]
  affects: [internal/deployer/infrastructure.go]
tech_stack:
  added: []
  patterns: [prefix-allowlist, nolint-truthful-suppression]
key_files:
  created: []
  modified:
    - internal/kube/manifest.go
    - internal/kube/manifest_test.go
decisions:
  - "fetchManifestAllowedPrefixes as package-level var makes the trust boundary testable and extensible without requiring HTTP mocking"
  - "nolint:gosec replaces #nosec G107 — the suppression is now truthful because URLs are validated before http.Get"
metrics:
  duration: 2m
  completed: "2026-03-10"
  tasks_completed: 2
  files_modified: 2
requirements_completed: [SEC-02]
---

# Phase 05 Plan 02: FetchManifest Domain Allowlist Summary

**One-liner:** Domain allowlist guard in FetchManifest rejects non-GitHub URLs before http.Get, replacing #nosec G107 with a truthful //nolint:gosec suppression.

## What Was Built

Added a URL allowlist guard to `FetchManifest` in `internal/kube/manifest.go`:

- `fetchManifestAllowedPrefixes` package-level var with two trusted prefixes: `https://github.com/` and `https://raw.githubusercontent.com/`
- Prefix-check loop at the top of `FetchManifest` rejects untrusted URLs before any `http.Get` call
- Error message contains "not from a trusted domain" for clear, testable rejection messages
- Replaced `#nosec G107` with `//nolint:gosec // URL validated against fetchManifestAllowedPrefixes` (truthful suppression)
- `TestFetchManifest_Allowlist` table-driven tests: 6 subtests covering blocked (arbitrary domain, http downgrade, empty string, github subdomain) and allowed (github.com, raw.githubusercontent.com) cases

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Add allowlist guard + tests (TDD RED→GREEN) | 089175d | internal/kube/manifest.go, internal/kube/manifest_test.go |
| 2 | Full unit suite passes | (no new commit - verification only) | - |

## Decisions Made

- `fetchManifestAllowedPrefixes` as a package-level var: testable (tests can inspect the var), extensible (add new prefixes without touching function body), matches the established pattern in `internal/validation/loader.go`
- No HTTP mock needed for rejected-URL tests: the guard returns before `http.Get`, so tests run fast without network or mock infrastructure
- The `else if` pattern used in test assertions to satisfy gocritic's `elseif` linting rule (auto-fixed inline)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] gocritic elseif lint warning in test code**
- **Found during:** Task 1 (lint step)
- **Issue:** `else { if cond {} }` pattern in `TestFetchManifest_Allowlist` triggered gocritic's `elseif` check
- **Fix:** Replaced `else { if err != nil { ... } }` with `else if err != nil { ... }`
- **Files modified:** internal/kube/manifest_test.go
- **Commit:** included in 089175d (pre-commit hook auto-formats)

## Self-Check

- [x] `internal/kube/manifest.go` — fetchManifestAllowedPrefixes var present
- [x] `internal/kube/manifest_test.go` — TestFetchManifest_Allowlist present
- [x] `#nosec` removed from manifest.go
- [x] `//nolint:gosec` added to manifest.go
- [x] All 7 TestFetchManifest_Allowlist subtests pass
- [x] Full unit suite exits 0 (all packages `ok`)
- [x] Commit 089175d exists

## Self-Check: PASSED
