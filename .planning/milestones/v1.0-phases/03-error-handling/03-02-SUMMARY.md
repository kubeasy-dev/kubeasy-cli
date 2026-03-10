---
phase: 03-error-handling
plan: 02
subsystem: infra
tags: [constants, env-var, configuration, testing]

# Dependency graph
requires: []
provides:
  - "func init() in internal/constants/const.go reads KUBEASY_API_URL at process start and overrides WebsiteURL"
  - "Two unit tests covering env var override and default retention behaviors"
affects:
  - "internal/api/client.go — reads constants.WebsiteURL at client construction time"
  - "Any future phase that uses constants.WebsiteURL"

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "os.Getenv-based init() override for compile-time defaults — env var wins over ldflags"

key-files:
  created: []
  modified:
    - internal/constants/const.go
    - internal/constants/const_test.go

key-decisions:
  - "KUBEASY_API_URL env var overrides WebsiteURL at process start via init() — env var priority beats GoReleaser ldflags to enable staging use without special builds"

patterns-established:
  - "init() override pattern: set default at var declaration, override in init() from env var — usable for other overridable constants"

requirements-completed:
  - ERR-03

# Metrics
duration: 2min
completed: 2026-03-09
---

# Phase 3 Plan 02: KUBEASY_API_URL Env Var Override Summary

**func init() in const.go reads KUBEASY_API_URL at startup and overrides WebsiteURL, with two unit tests confirming env var priority over compile-time defaults**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-09T15:54:18Z
- **Completed:** 2026-03-09T15:56:00Z
- **Tasks:** 1 (TDD: 2 commits — test + feat)
- **Files modified:** 2

## Accomplishments
- Added `os` import and `func init()` to `internal/constants/const.go` that reads `KUBEASY_API_URL` and overrides `WebsiteURL` when set
- Added `TestWebsiteURL_EnvOverride` confirming env var sets WebsiteURL to staging URL
- Added `TestWebsiteURL_NoEnv_Retains_Default` confirming default is preserved when env var is absent
- Full unit test suite (`task test:unit`) remains green at 46.1% coverage

## Task Commits

Each task was committed atomically (TDD):

1. **RED — WebsiteURL env var tests** - `d376d51` (test)
2. **GREEN — init() implementation** - `d7e86ba` (feat)

_Note: TDD task uses two commits (test then feat). No refactor needed — implementation is minimal._

## Files Created/Modified
- `internal/constants/const.go` - Added `"os"` import and `func init()` that overrides WebsiteURL from KUBEASY_API_URL
- `internal/constants/const_test.go` - Added TestWebsiteURL_EnvOverride and TestWebsiteURL_NoEnv_Retains_Default

## Decisions Made
- KUBEASY_API_URL env var overrides WebsiteURL — this gives env var priority over GoReleaser ldflags, enabling developers to target staging with `KUBEASY_API_URL=https://staging.kubeasy.com go run main.go` without a special build

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None — the test file already existed with other tests; new tests were appended to it.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- ERR-03 complete: WebsiteURL is now configurable via env var at runtime
- Ready for phase 03 plan 03 (ERR-01, ERR-02 if remaining)

---
*Phase: 03-error-handling*
*Completed: 2026-03-09*
