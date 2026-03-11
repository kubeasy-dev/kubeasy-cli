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

---

## Milestone: v2.7.0 — Connectivity Extension

**Shipped:** 2026-03-11
**Phases:** 4 | **Plans:** 10

### What Was Built

- `SetupAllComponents` with 6 idempotent component installers and per-component status output
- Kind cluster extraPortMappings 8080/8443 with full config-diff detection and recreation prompt
- CLI-managed probe pod lifecycle (`curlimages/curl`) — auto-deploy/cleanup for NetworkPolicy testing
- Blocked-connection assertion (`expectedStatus: 0`) + `sourceNamespace` cross-namespace field
- External HTTP mode (`mode: external`) via `net/http` with Host header override and sslip.io support
- TLS certificate validation (expiry, SAN, insecureSkipVerify) via `tls.Dialer` — pure stdlib

### What Worked

- **TDD RED→GREEN throughout**: All 4 phases wrote failing tests first. The RED phase was non-negotiable — it caught spec ambiguities before implementation started.
- **Parallel plan execution (phases 06-02 and 06-03)**: Two plans ran concurrently in separate worktrees. Delivered two installers in the time it would have taken one.
- **Probe pod independent-context cleanup**: Using `context.Background()` with a 10s timeout for cleanup (not the caller's context) was the right call — PROBE-03 works correctly with Ctrl-C.
- **mode discriminant over new ValidationType**: Avoiding a new type prevented breaking changes across three repos. Parse-time validation in loader.go made the constraint enforceable.
- **sslip.io for external routing**: Zero DNS configuration required — the URL encodes the IP. Worked immediately for localhost (127.x.x.x) with no special handling.

### What Was Inefficient

- **Nyquist sign-off deferred again**: All 4 phases executed with `nyquist_compliant: false`. Pattern from v1.0 repeated. The `/gsd:validate-phase` step needs to be part of the execute-phase checklist, not a post-milestone cleanup.
- **SUMMARY.md `requirements-completed` frontmatter missed in 06-02 and 06-03**: The parallel execution caused attribution confusion. Both plans wrote code but only one got the SUMMARY frontmatter. Needs a checklist item in execute-phase.
- **`errNoSourcePodSpecified` constant left as dead code**: Phase 07 plan noted to remove it but didn't — orphaned constant survived to audit. Small but recurring pattern of "clean up noted, not done."
- **Phase 08 `IP auto-resolution from Ingress/Gateway resources`**: This was never actually implemented (the feature uses sslip.io hostnames instead). The roadmap goal mentioned it but the implementation took the simpler sslip.io path. ROADMAP goals should be updated when approach changes mid-phase.

### Patterns Established

- **ComponentResult pattern**: `func install*(ctx, clientset, ...) ComponentResult` — check readiness first, install if needed, always return a result. Reusable for future components.
- **Two-pass apply for CRD-dependent resources**: Apply CRDs → rebuild REST mapper → apply dependent resources. Required for Gateway API (and cert-manager). Pattern documented in 06-02 SUMMARY.
- **httptest.NewServer for external connectivity tests**: No real HTTP server needed in tests — `httptest.NewServer` handles TLS, status codes, and headers cleanly.
- **tls.Dialer for cert probing**: Separating TLS handshake (cert inspection) from HTTP request allows validating cert properties independently of response status.
- **Adapter pattern for testability**: `writeKindConfig()` → `writeKindConfigToPath(path)`. Public adapter calls the testable path-parameterized variant. Used for Kind config I/O.

### Key Lessons

1. **Make `/gsd:validate-phase` part of execute-phase, not post-milestone cleanup** — two milestones in a row with deferred Nyquist sign-off is the pattern failing.
2. **Update ROADMAP phase goals when implementation approach changes** — "IP auto-resolution from Ingress/Gateway" was never built; sslip.io was the actual approach. The goal should have been updated when the decision was made.
3. **Parallel plan execution requires coordination on SUMMARY frontmatter** — when two plans touch the same file in the same commit, both need their `requirements-completed` fields populated before commit.
4. **sslip.io 127.x.x.x on macOS Docker Desktop needs a warning** — EXT-03 runtime risk is real; a CLI warning when `mode: external` targets are used would surface this to challenge authors early.

### Cost Observations

- Sessions: 1 milestone, 4 phases, 10 plans
- Execution time: ~1 day (includes research, planning, execution, verification, audit)
- Model mix: balanced profile (sonnet throughout)
- Notable: Phase 06 was the heaviest (4 plans, parallel execution) but delivered the most infrastructure surface area

---

## Cross-Milestone Trends

| Metric | v1.0 | v2.7.0 |
|--------|------|--------|
| Phases | 5 | 4 |
| Plans | 14 | 10 |
| Requirements | 16/16 | 20/20 |
| Test coverage | ~45.8% | ~37.3% (unit) |
| Audit status | tech_debt (no blockers) | tech_debt (no blockers) |
| Timeline | 2 days | 1 day |
| Nyquist at ship | 0/5 compliant | 1/4 compliant (Phase 07 only) |
| Tech debt pattern | deferred Nyquist | deferred Nyquist + stale test |
