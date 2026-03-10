# Phase 3: Error Handling - Context

**Gathered:** 2026-03-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Surface errors from manifest application to the user, enable Ctrl-C cancellation on all API calls, and allow a `KUBEASY_API_URL` env var to override the backend URL for local builds without GoReleaser.

</domain>

<decisions>
## Implementation Decisions

### ApplyManifest error policy
- Fail on first critical error — return the first non-recoverable error immediately and stop the loop
- **Critical** (stop and return): create failures, update failures
- **Skippable** (log and continue): decode errors, API-not-found (`IsNotFound` or "server could not find"), RESTMapping failures (unknown GVK)
- The comment `// Return nil even if some documents failed` is removed; function now returns the first critical error
- Caller (`challenge start`) returns the error to the user with a clear message — non-zero exit code, no silent success

### Context propagation scope
- All public API functions in `internal/api/client.go` get a `ctx context.Context` parameter
- Compat aliases (GetChallenge, GetChallengeProgress, etc.) also updated to accept ctx — they are NOT removed yet (QUAL-01 handles that in Phase 4)
- All `cmd/` RunE handlers pass `cmd.Context()` to every api.* call (start, submit, reset, clean)
- `context.Background()` is removed from all HTTP-calling functions in client.go — replaced by the passed ctx
- Cobra cancels `cmd.Context()` on Ctrl-C automatically — no extra signal handling needed

### API URL env var
- `KUBEASY_API_URL` env var is read in a `func init()` in `internal/constants/const.go`
- Priority: env var > ldflags > default (`http://localhost:3000`)
- Only `WebsiteURL` is affected — GitHub URLs and download URLs stay fixed
- Pattern: `if v := os.Getenv("KUBEASY_API_URL"); v != "" { WebsiteURL = v }`

### Claude's Discretion
- Exact error message format for manifest failures (e.g., wrapping with resource name/kind)
- Whether to add a test for the KUBEASY_API_URL init behavior or rely on the existing client_http_test pattern

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `cmd.Context()` in Cobra RunE: already used in `cmd/start.go:69` — pattern to replicate in submit, reset, clean
- `apierrors.IsAlreadyExists`, `apierrors.IsNotFound`: already imported in `manifest.go` — use for the critical vs skippable distinction
- `client_http_test.go`: has `overrideServerURL` helper — existing pattern for testing URL overrides

### Established Patterns
- Function vars (e.g., `apiGetChallenge = api.GetChallenge`) in cmd/ — signature change in api.* propagates to all function var declarations
- `var WebsiteURL = "http://localhost:3000"` is a `var` (not `const`), already overridden by GoReleaser ldflags — init() override follows the same mechanism

### Integration Points
- `internal/kube/manifest.go` `ApplyManifest`: error return path needs to be added; callers in deployer need to handle the new non-nil return
- `internal/api/client.go`: all public functions get ctx param; `apigen`-generated client methods already accept ctx (they call `WithResponse(ctx, ...)` patterns)
- `cmd/start.go`, `cmd/submit.go`, `cmd/reset.go`, `cmd/clean.go`: function var declarations and RunE bodies need ctx threading

</code_context>

<specifics>
## Specific Ideas

- The success criterion states: "exits with a non-zero code and a user-visible error message — not silent success" — error must reach the user's terminal, not just the logger
- Ctrl-C during `kubeasy challenge start` or `submit` cancels within one second — this is satisfied by passing cmd.Context() since Cobra signals it immediately on interrupt
- `KUBEASY_API_URL=https://staging.kubeasy.com go run main.go challenge get <slug>` must reach staging — the init() approach satisfies this exactly

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 03-error-handling*
*Context gathered: 2026-03-09*
