# Retrospective: kubeasy-cli

## Milestone: v1.0 — Réduction de la dette technique

**Shipped:** 2026-03-11
**Phases:** 5 | **Plans:** 14

### What Was Built

- Panic-safe executor with comma-ok assertions across 5 validation types
- Unit test suite for all 4 core commands via function-var injection (11 tests)
- ApplyManifest fail-fast error propagation — manifests no longer silently succeed
- Full context propagation through 17 api.* functions (Ctrl-C works)
- KUBEASY_API_URL env var for local dev without GoReleaser builds
- API alias cleanup — 6 duplicates removed, single canonical names throughout
- Shared applyManifestDirs helper — walk-and-apply duplication eliminated
- PollUntilContextTimeout replacing manual sleep loops in readiness polling
- Shell injection eliminated in connectivity validation (buildCurlCommand)
- FetchManifest domain allowlist (trusted GitHub URLs only)

### What Worked

- **Strict phase ordering**: Each phase left the codebase fully functional and lint-green. No cascading failures.
- **TDD red-green pattern (Phase 1)**: Writing RED tests first made the safety requirements concrete before touching production code.
- **Function-var injection**: Clean testability pattern without interfaces — low overhead, high coverage gain.
- **Atomic commits for interdependent tasks**: Phase 4 QUAL-02 committed tasks 1+2 together because golangci-lint rejected unexported functions with no callers.
- **Explicit TODO(sec) annotation**: Deferring the wget fallback with a named comment prevents it from getting lost.

### What Was Inefficient

- **ROADMAP.md plan checkboxes not updated during execution** — all plans showed `[ ]` at completion; required manual fix during milestone archive. Automation gap.
- **SUMMARY.md `one_liner` field missing** — gsd-tools summary-extract returned null for all files; accomplishments had to be inferred from VERIFICATION content. Adding one_liner to SUMMARY template would fix this.
- **STATE.md stale metrics** — Performance metrics section only partially updated (showed Phase 1 data in velocity header despite 14 plans completing). A known limitation of the state update mechanism.
- **Nyquist VALIDATION.md files created but never completed** — All 4 existing ones have `nyquist_compliant: false`. Phase 4 has none. These could be addressed during execute-phase rather than as a separate step.

### Patterns Established

- **Function-var injection pattern**: Declare `var apiGetChallenge = api.GetChallengeBySlug` at package level; tests reassign in save/restore pattern. Used in start.go, submit.go, reset.go.
- **ui.SetCIMode(true) in TestMain**: Required for -race clean tests when pterm spinners are in production code paths.
- **Slug validation as first RunE statement**: Before any ui.Section or API call — provides fail-fast behavior and clean testability without mocks.
- **init() for env var overrides**: `func init()` in constants package reads KUBEASY_API_URL before any Cobra command runs.
- **//nolint:gosec with explanation**: Replacing `#nosec` with `//nolint:gosec // URL validated against fetchManifestAllowedPrefixes` makes suppressions truthful.

### Key Lessons

1. **Validate ROADMAP.md plan checkboxes match SUMMARY.md existence** — the disconnect caused confusion during audit.
2. **Add `one_liner` to SUMMARY.md template** — gsd-tools needs it; currently the field is absent from all plan summaries.
3. **Consider `/gsd:validate-phase` immediately after execute-phase** — Nyquist compliance is easier to achieve while the phase is fresh.
4. **Phase size was appropriate** — 2-3 plans per phase kept each phase reviewable. Larger phases risked cross-plan interference.

### Cost Observations

- Sessions: 1 milestone, 5 phases, 14 plans
- Execution time: ~65 minutes total (4m–8m per plan)
- Model mix: balanced profile (sonnet throughout)
- Notable: Security phases (4, 5) were fastest — clear requirements, minimal surface area

---

## Cross-Milestone Trends

| Metric | v1.0 |
|--------|------|
| Phases | 5 |
| Plans | 14 |
| Requirements | 16/16 |
| Test coverage | ~45.8% |
| Audit status | tech_debt (no blockers) |
| Timeline | 2 days |
