---
phase: 08-external-http
plan: 02
subsystem: validation
tags: [net/http, connectivity, external, httptest, tdd]

# Dependency graph
requires:
  - phase: 08-external-http/08-01
    provides: ConnectivitySpec.Mode + ConnectivityCheck.HostHeader fields in types.go
provides:
  - checkExternalConnectivity method in executor.go using net/http (no pod exec)
  - checkExternalConnectivityAll helper iterating targets for external mode
  - executeConnectivity mode branch routing mode:external to net/http path
affects: [challenges using connectivity mode:external, validation executor tests]

# Tech tracking
tech-stack:
  added: [net/http (stdlib — already available, newly imported in executor.go)]
  patterns:
    - "External connectivity uses per-request http.Client with CheckRedirect=ErrUseLastResponse"
    - "req.Host override for virtual-host routing (not req.Header.Set)"
    - "context.WithTimeout wraps caller ctx for per-request deadline"
    - "expectedStatusCode==0 signals blocked-as-expected for both internal and external modes"

key-files:
  created: []
  modified:
    - internal/validation/executor.go
    - internal/validation/executor_test.go

key-decisions:
  - "net/http imported in executor.go (stdlib only) — no new dependencies added"
  - "TDD RED+GREEN committed atomically (pre-commit hook blocks RED-only commits)"
  - "req.Host used (not req.Header.Set) — only req.Host overrides Host wire header in Go"
  - "CheckRedirect returns http.ErrUseLastResponse — allows 3xx assertions in challenge specs"

patterns-established:
  - "External mode branch before SourcePod resolution — mode:external never touches K8s clients"
  - "Per-target timeout from TimeoutSeconds, fallback to DefaultConnectivityTimeoutSeconds"

requirements-completed: [EXT-01, EXT-02, EXT-03, EXT-04]

# Metrics
duration: 3min
completed: 2026-03-11
---

# Phase 8 Plan 02: External HTTP Connectivity Summary

**net/http-based external connectivity executor with Host header override and 13 new unit tests via httptest.NewServer**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-11T13:00:05Z
- **Completed:** 2026-03-11T13:03:30Z
- **Tasks:** 2 (TDD RED+GREEN)
- **Files modified:** 2

## Accomplishments
- `checkExternalConnectivity`: sends HTTP GET from CLI host, handles connection errors, checks status code match
- `checkExternalConnectivityAll`: iterates all targets, collects failure messages, returns `msgAllConnectivityPassed` on full pass
- `executeConnectivity` branches on `spec.Mode == "external"` before any SourcePod/probe logic
- 13 new unit tests (7 functions) using `httptest.NewServer` — deterministic, no DNS, no cluster needed
- Full unit suite continues to pass (0 regressions in 310+ tests)

## Task Commits

1. **Task 1 + 2: External connectivity implementation (TDD RED+GREEN)** - `14942d8` (feat)

_Note: RED and GREEN committed atomically because the pre-commit lint hook rejects compilation failures._

## Files Created/Modified
- `internal/validation/executor.go` - Added `checkExternalConnectivity`, `checkExternalConnectivityAll`, mode branch in `executeConnectivity`, imported `net/http`
- `internal/validation/executor_test.go` - Added 7 test functions (13 test cases) for external connectivity; added `fmt`, `net/http`, `net/http/httptest` imports

## Decisions Made
- RED+GREEN committed atomically: the pre-commit hook runs `golangci-lint` which compiles test files; a RED-only commit fails the hook. This is expected behavior for this project — TDD tests and implementation committed together.
- `req.Host` used for virtual-host routing (not `req.Header.Set("Host", ...)`): Go's http.Client ignores Host set via Header.Set; only `req.Host` overrides the wire Host header.
- `http.ErrUseLastResponse` in `CheckRedirect`: allows challenge specs to assert on 3xx status codes without automatic redirect following.

## Deviations from Plan

None - plan executed exactly as written. The only deviation from the TDD protocol (committing RED before GREEN) was forced by the pre-commit hook, which is a project convention, not a plan deviation.

## Issues Encountered
- Pre-commit hook blocks RED-only TDD commits (golangci-lint compiles test files). Solution: committed RED+GREEN atomically after GREEN was confirmed.

## Next Phase Readiness
- External HTTP connectivity fully implemented and tested
- EXT-01 through EXT-04 requirements satisfied
- Phase 8 complete — both plans (loader types and executor implementation) delivered

---
*Phase: 08-external-http*
*Completed: 2026-03-11*
