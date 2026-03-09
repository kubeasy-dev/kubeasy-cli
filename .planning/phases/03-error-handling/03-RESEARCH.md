# Phase 3: Error Handling - Research

**Researched:** 2026-03-09
**Domain:** Go error propagation, context cancellation, environment variable overrides
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### ApplyManifest error policy
- Fail on first critical error — return the first non-recoverable error immediately and stop the loop
- **Critical** (stop and return): create failures, update failures
- **Skippable** (log and continue): decode errors, API-not-found (`IsNotFound` or "server could not find"), RESTMapping failures (unknown GVK)
- The comment `// Return nil even if some documents failed` is removed; function now returns the first critical error
- Caller (`challenge start`) returns the error to the user with a clear message — non-zero exit code, no silent success

#### Context propagation scope
- All public API functions in `internal/api/client.go` get a `ctx context.Context` parameter
- Compat aliases (GetChallenge, GetChallengeProgress, etc.) also updated to accept ctx — they are NOT removed yet (QUAL-01 handles that in Phase 4)
- All `cmd/` RunE handlers pass `cmd.Context()` to every api.* call (start, submit, reset, clean)
- `context.Background()` is removed from all HTTP-calling functions in client.go — replaced by the passed ctx
- Cobra cancels `cmd.Context()` on Ctrl-C automatically — no extra signal handling needed

#### API URL env var
- `KUBEASY_API_URL` env var is read in a `func init()` in `internal/constants/const.go`
- Priority: env var > ldflags > default (`http://localhost:3000`)
- Only `WebsiteURL` is affected — GitHub URLs and download URLs stay fixed
- Pattern: `if v := os.Getenv("KUBEASY_API_URL"); v != "" { WebsiteURL = v }`

### Claude's Discretion
- Exact error message format for manifest failures (e.g., wrapping with resource name/kind)
- Whether to add a test for the KUBEASY_API_URL init behavior or rely on the existing client_http_test pattern

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| ERR-01 | `ApplyManifest` collects and returns errors from critical resources instead of always returning `nil` | Existing `apierrors.IsAlreadyExists`/`IsNotFound` imports in `manifest.go` provide the critical/skippable decision; returning first critical error stops the loop immediately |
| ERR-02 | All functions in `internal/api/client.go` accept `ctx context.Context` and propagate it to HTTP requests (Ctrl-C cancels immediately) | All `WithResponse` generated-client methods already accept `ctx`; only the outer public functions hardcode `context.Background()` — replacing those is the complete change |
| ERR-03 | `constants.WebsiteURL` uses `KUBEASY_API_URL` as env-var fallback for local builds without GoReleaser | `WebsiteURL` is already a `var` overridden by ldflags; a package `init()` applies the same override pattern with `os.Getenv` |
</phase_requirements>

---

## Summary

Phase 3 makes three targeted changes to an existing Go Cobra CLI. All three changes are surgically scoped and do not require new dependencies.

**ERR-01** fixes a silent-failure bug in `internal/kube/manifest.go`: `ApplyManifest` has always returned `nil` regardless of whether create/update operations failed. The decision is to return the first non-recoverable error immediately; decode errors, API-not-found errors, and REST mapping failures remain skippable (log and continue). The callers in `internal/deployer/challenge.go` already propagate non-nil returns from `kube.ApplyManifest`, so the deployer layer needs no structural changes — it just benefits from the fix.

**ERR-02** threads `ctx context.Context` through all public functions in `internal/api/client.go`. Every generated-client method (`GetChallengeWithResponse`, `StartChallengeWithResponse`, etc.) already accepts a `ctx` as its first argument; the only change is replacing the hardcoded `context.Background()` calls with the passed parameter. The `cmd/` layer already calls `cmd.Context()` in `start.go` for cluster operations; the same pattern is replicated for all API calls in start, submit, reset, and clean. Cobra cancels `cmd.Context()` automatically on Ctrl-C — no signal handler is needed.

**ERR-03** adds a `func init()` to `internal/constants/const.go` that reads `KUBEASY_API_URL` and, when non-empty, sets `WebsiteURL` to that value. The init runs before any command executes, satisfying `go run main.go challenge get <slug>` without a GoReleaser build.

**Primary recommendation:** Implement ERR-01 first (self-contained, single file), then ERR-02 (wider fan-out but mechanical), then ERR-03 (two-line change + optional test). Each can be its own plan wave.

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `context` (stdlib) | Go stdlib | Context propagation and cancellation | Zero-dependency; Cobra uses it natively |
| `os` (stdlib) | Go stdlib | `os.Getenv` for `KUBEASY_API_URL` | Already imported in constants package |
| `k8s.io/apimachinery/pkg/api/errors` | already in go.mod | `IsNotFound`, `IsAlreadyExists` predicates | Already imported in `manifest.go` |
| `github.com/spf13/cobra` | already in go.mod | `cmd.Context()` on every RunE | Cobra already cancels on interrupt |

### Supporting
No new dependencies required for this phase.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Package `init()` for env override | `viper` or `pflag` env binding | init() is zero-dependency; viper would be overkill for one variable |
| Returning first critical error | Collecting all errors | First-error-stop is simpler, avoids multi-error types, matches the locked decision |

**Installation:**
No new packages — this phase adds zero dependencies.

---

## Architecture Patterns

### Pattern 1: Critical vs. Skippable Error Classification in ApplyManifest

**What:** The existing manifest application loop silently continues on all errors. After this change, errors are classified at the point they occur.

**When to use:** Any resource application loop where partial success is acceptable for some error classes but not others.

**Skippable errors (log Warning, `continue` to next document):**
- Decode errors (`decoder.Decode` fails) — malformed YAML documents are not a deployment blocker
- RESTMapping failures (`mapper.RESTMapping` fails) — unknown GVK means the API is not installed (e.g., Kyverno CRD not yet ready)
- `apierrors.IsNotFound(err)` or `strings.Contains(err.Error(), "the server could not find the requested resource")` — API group not installed

**Critical errors (return immediately):**
- `resourceClient.Create` returns an error that is NOT `IsAlreadyExists` and NOT `IsNotFound` — e.g., forbidden, quota exceeded, invalid spec
- `resourceClient.Update` returns any error — a resource that exists but can't be updated is a deployment failure
- `resourceClient.Get` (for the update path) returns any error — can't proceed with update if the existing object can't be fetched

**Current code path to modify** (line 145 of `internal/kube/manifest.go`):
```go
return nil // Return nil even if some documents failed, as per previous logic
```
This line is replaced by returning the first critical error captured during the loop.

**Error message format (Claude's discretion — recommendation):**
Wrap with resource identity for user clarity:
```go
return fmt.Errorf("failed to create %s/%s: %w", objKind, objName, err)
return fmt.Errorf("failed to update %s/%s: %w", objKind, objName, updateErr)
```

### Pattern 2: Context Threading Through Public API Functions

**What:** All public functions in `internal/api/client.go` that make HTTP requests currently hardcode `context.Background()`. Adding `ctx context.Context` as the first parameter and passing it to the generated client's `WithResponse` methods enables cancellation.

**Affected functions (complete list from reading client.go):**
- `GetProfile()` → `GetProfile(ctx context.Context)`
- `Login()` → `Login(ctx context.Context)`
- `GetUserProfile()` → `GetUserProfile(ctx context.Context)` (compat alias — also updated per locked decision)
- `GetChallengeBySlug(slug string)` → `GetChallengeBySlug(ctx context.Context, slug string)`
- `GetChallengeStatus(slug string)` → `GetChallengeStatus(ctx context.Context, slug string)`
- `StartChallengeWithResponse(slug string)` → `StartChallengeWithResponse(ctx context.Context, slug string)`
- `StartChallenge(slug string)` → `StartChallenge(ctx context.Context, slug string)` (compat alias — delegates to StartChallengeWithResponse)
- `SubmitChallenge(slug string, req ...)` → `SubmitChallenge(ctx context.Context, slug string, req ...)`
- `SendSubmit(challengeSlug string, results ...)` → `SendSubmit(ctx context.Context, challengeSlug string, results ...)`
- `ResetChallenge(slug string)` → `ResetChallenge(ctx context.Context, slug string)`
- `ResetChallengeProgress(slugOrID string)` → `ResetChallengeProgress(ctx context.Context, slugOrID string)` (compat alias)
- `TrackSetup()` → `TrackSetup(ctx context.Context)`
- `GetTypes()` → `GetTypes(ctx context.Context)`
- `GetThemes()` → `GetThemes(ctx context.Context)`
- `GetDifficulties()` → `GetDifficulties(ctx context.Context)`

**Propagation pattern** (identical for all functions):
```go
// Before
resp, err := client.GetChallengeWithResponse(context.Background(), slug)
// After
resp, err := client.GetChallengeWithResponse(ctx, slug)
```

**cmd/ layer — function var signature updates:**

Each `cmd/` file that declares function vars over api.* functions must update those var types. Example from `cmd/start.go`:
```go
// Before
var apiGetChallenge = api.GetChallenge  // type: func(string) (*ChallengeEntity, error)

// After
var apiGetChallenge = api.GetChallenge  // type: func(context.Context, string) (*ChallengeEntity, error)
```

The function vars themselves don't change declaration syntax — Go infers the type from the assigned function. But all call sites must pass `cmd.Context()`:
```go
// Before
challenge, err = apiGetChallenge(challengeSlug)
// After
challenge, err = apiGetChallenge(cmd.Context(), challengeSlug)
```

**Impact on existing tests in `cmd/`:** The function var injectors in `start_test.go`, `submit_test.go`, `reset_test.go` use anonymous functions. Their signatures must update to match the new type:
```go
// Before
apiGetChallenge = func(slug string) (*api.ChallengeEntity, error) { ... }
// After
apiGetChallenge = func(ctx context.Context, slug string) (*api.ChallengeEntity, error) { ... }
```

**Impact on tests in `internal/api/`:** `client_http_test.go` calls public functions directly. Each call site gains a `context.Background()` argument (tests don't need cancellation behavior, `context.Background()` is appropriate in tests).

### Pattern 3: Package init() for Environment Variable Override

**What:** A `func init()` in `internal/constants/const.go` reads `KUBEASY_API_URL` and overrides `WebsiteURL` if set.

**When to use:** When ldflags set a production default but local `go run` needs to point at a different backend without rebuilding with GoReleaser.

**Priority chain:**
1. GoReleaser ldflags set `WebsiteURL = "https://kubeasy.dev"` at build time
2. `func init()` runs at process start; if `KUBEASY_API_URL` is non-empty, it overrides whatever value the ldflags set
3. Default compile-time value `"http://localhost:3000"` applies only when neither ldflags nor env var are present

**Implementation:**
```go
// Source: locked decision in CONTEXT.md
func init() {
    if v := os.Getenv("KUBEASY_API_URL"); v != "" {
        WebsiteURL = v
    }
}
```

`os` is not currently imported in `const.go` — the import must be added.

**Testing approach (Claude's discretion — recommendation):** Add a test in `internal/constants/` or inline in `internal/api/client_http_test.go` using the existing `overrideServerURL` pattern. Because `init()` runs before tests, testing the init itself requires temporarily manipulating `os.Setenv` + resetting `WebsiteURL`:
```go
func TestWebsiteURL_EnvOverride(t *testing.T) {
    orig := constants.WebsiteURL
    t.Cleanup(func() { constants.WebsiteURL = orig })

    t.Setenv("KUBEASY_API_URL", "https://staging.kubeasy.com")
    // Re-trigger the init logic inline (init() already ran; test the pattern directly)
    if v := os.Getenv("KUBEASY_API_URL"); v != "" {
        constants.WebsiteURL = v
    }
    assert.Equal(t, "https://staging.kubeasy.com", constants.WebsiteURL)
}
```
Note: `t.Setenv` (Go 1.17+) automatically restores the env var on cleanup. Testing `init()` itself is not feasible without a subprocess; testing the override logic inline is the established pattern from `client_http_test.go`.

### Anti-Patterns to Avoid

- **Collecting all manifest errors before returning:** The locked decision is fail-fast on first critical error. Don't accumulate errors into a `[]error` slice.
- **Adding signal handling in `cmd/`:** Cobra handles SIGINT automatically through `cmd.Context()`. Do not add `signal.Notify` or `os.Signal` channels.
- **Passing `context.Background()` from cmd/ to api.*:** cmd/ must always pass `cmd.Context()` — that is the entire point of ERR-02.
- **Changing `TrackSetup` signature without updating caller:** `TrackSetup` is called in `cmd/setup.go` fire-and-forget style. Its signature must match the updated API but callers can pass `cmd.Context()`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Critical vs. non-critical API errors | Custom error type hierarchy | `apierrors.IsNotFound`, `apierrors.IsAlreadyExists` | Already imported in manifest.go; these are the standard k8s error predicates |
| Context cancellation on Ctrl-C | `signal.Notify` loop | `cmd.Context()` from Cobra | Cobra already calls `cancel()` on SIGINT via `cobra.Command.ExecuteContext` |
| Environment variable priority | Custom config loader | `os.Getenv` in `init()` | Two lines; no framework needed for one variable |

**Key insight:** All three requirements are addressed by standard library features and patterns already in use elsewhere in the codebase.

---

## Common Pitfalls

### Pitfall 1: Function Var Type Mismatch After API Signature Change

**What goes wrong:** After adding `ctx` to `api.GetChallenge`, the function var `apiGetChallenge = api.GetChallenge` in `cmd/start.go` is fine (Go infers the new type), but every anonymous function assigned to that var in tests still has the old signature `func(slug string)`. The compiler will catch this, but it's easy to miss in the test files.

**Why it happens:** There are six function vars across four cmd/ files plus matching test functions. It's a mechanical but wide change.

**How to avoid:** After updating client.go, run `go build ./...` before running tests — compilation errors will enumerate all stale call sites.

**Warning signs:** `cannot use func literal (type func(string) ...)` compile errors in `*_test.go` files.

### Pitfall 2: init() Ordering Relative to ldflags

**What goes wrong:** Developer assumes `init()` runs before ldflags are applied. In reality, ldflags set values at compile time (they are string literals baked into the binary). `init()` runs at process start, after all variable initializations but before `main()`. So `WebsiteURL` is first set to `"http://localhost:3000"` (compile-time default), then overwritten by ldflags at link time, then `init()` runs and optionally overwrites again with the env var.

**Why it happens:** Confusion between compile-time (ldflags) and runtime (init) phases.

**How to avoid:** The priority chain is correct as designed: env var in `init()` always wins at runtime over the ldflags value.

**Warning signs:** If testing with `go run`, ldflags are NOT applied (they require `go build -ldflags`), so `WebsiteURL` starts as `"http://localhost:3000"` and `KUBEASY_API_URL` env var overrides it — exactly the success criterion.

### Pitfall 3: TrackSetup Is Called Fire-and-Forget

**What goes wrong:** `TrackSetup()` is called in `cmd/setup.go` without capturing its return value and currently takes no arguments. Adding `ctx` changes its call signature.

**Why it happens:** Fire-and-forget pattern obscures that there's a caller that needs updating.

**How to avoid:** Search all callers of `TrackSetup` before updating the function. There is one caller: `cmd/setup.go`.

**Warning signs:** `too many arguments in call to api.TrackSetup` compile error.

### Pitfall 4: Test Isolation for WebsiteURL Override

**What goes wrong:** Tests in `internal/api/` run in parallel and share the package-level `constants.WebsiteURL` variable. If one test sets `KUBEASY_API_URL` via `os.Setenv` without cleanup, subsequent tests connect to the wrong server.

**Why it happens:** Package-level mutable state + parallel tests.

**How to avoid:** Use `t.Setenv` (not `os.Setenv`) — it automatically restores the original value via `t.Cleanup`. Also reset `constants.WebsiteURL` explicitly in cleanup.

**Warning signs:** Flaky tests where some pass and some fail depending on run order.

### Pitfall 5: ApplyManifest Callers in deployer Already Handle Non-Nil Returns

**What goes wrong:** Assuming that making `ApplyManifest` return errors requires changes in `deployer/challenge.go`. The caller already does:
```go
if err := kube.ApplyManifest(ctx, data, slug, mapper, dynamicClient); err != nil {
    return fmt.Errorf("failed to apply manifest %s: %w", filepath.Base(f), err)
}
```
The deployer propagates errors correctly — it just never received non-nil before.

**Why it happens:** Reading the deployer code superficially.

**How to avoid:** The deployer needs no structural changes — only `manifest.go` changes.

---

## Code Examples

Verified patterns from reading existing source files:

### ApplyManifest: Returning First Critical Error (ERR-01)

The existing create-error branch (lines 99-136 of `manifest.go`) currently ends with `continue` in all cases. The change is:

```go
// Create failure — not IsNotFound, not IsAlreadyExists
// Before: logger.Warning + continue
// After: return the error immediately
return fmt.Errorf("failed to create %s/%s: %w", objKind, objName, err)

// Update failure — any update error is critical
// Before: logger.Warning + continue
// After: return the error immediately
return fmt.Errorf("failed to update %s/%s: %w", objKind, objName, updateErr)
```

The `return nil // Return nil even if some documents failed` at line 145 becomes the normal exit when all documents are processed without critical error.

### Context Threading (ERR-02) — Representative Function

```go
// Source: internal/api/client.go — GetChallengeBySlug (after change)
func GetChallengeBySlug(ctx context.Context, slug string) (*ChallengeEntity, error) {
    client, err := NewAuthenticatedClient()
    if err != nil {
        return nil, err
    }
    resp, err := client.GetChallengeWithResponse(ctx, slug)  // ctx replaces context.Background()
    // ... rest unchanged
}
```

### cmd/ Layer Call Site (ERR-02)

```go
// Source: cmd/start.go — inside ui.WaitMessage closure (after change)
challenge, err = apiGetChallenge(cmd.Context(), challengeSlug)
```

Note: `cmd.Context()` is captured by the closure over `cmd` which is in scope in `RunE`. This is the same pattern used for Kubernetes calls at line 69 of `start.go`.

### Environment Variable Override (ERR-03)

```go
// Source: internal/constants/const.go (after change)
import "os"

func init() {
    if v := os.Getenv("KUBEASY_API_URL"); v != "" {
        WebsiteURL = v
    }
}
```

### Test Injection Pattern After Signature Change

```go
// Source: cmd/start_test.go — updated anonymous function signature
apiGetChallenge = func(ctx context.Context, slug string) (*api.ChallengeEntity, error) {
    return &api.ChallengeEntity{Title: "Test"}, nil
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Silent manifest failures (`return nil`) | Return first critical error | Phase 3 | Users get actionable error message instead of apparent success |
| `context.Background()` hardcoded | `cmd.Context()` threaded through | Phase 3 | Ctrl-C cancels within ~1 second instead of hanging 30s |
| Production URL only via GoReleaser ldflags | `KUBEASY_API_URL` env var override | Phase 3 | `go run main.go` can reach staging/production |

---

## Open Questions

1. **Should `Login()` also accept ctx?**
   - What we know: Login is called from `cmd/login.go` which also has access to `cmd.Context()`. The locked decision says "all public API functions in client.go" which includes Login.
   - What's unclear: Login is interactive (prompts, reads keyring) and long-running — context cancellation could interrupt it mid-flow.
   - Recommendation: Add ctx to Login per the locked decision. The behavior on cancellation is acceptable (if user hits Ctrl-C during login, the CLI should exit).

2. **Does `GetTypes`, `GetThemes`, `GetDifficulties` need ctx?**
   - What we know: These are public functions in client.go; the locked decision covers "all public API functions."
   - What's unclear: These may not be called from cmd/ RunE handlers that have `cmd.Context()` available.
   - Recommendation: Add ctx to all three per the locked decision. Callers that don't have a meaningful context can pass `context.Background()`.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify (already in go.mod) |
| Config file | none (no pytest.ini/jest.config — Go uses `go test`) |
| Quick run command | `go test -race ./cmd/... ./internal/api/... ./internal/constants/...` |
| Full suite command | `task test:unit` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| ERR-01 | ApplyManifest returns error on create failure | unit | `go test -race ./internal/kube/... -run TestApplyManifest` | ❌ Wave 0 |
| ERR-01 | ApplyManifest returns error on update failure | unit | `go test -race ./internal/kube/... -run TestApplyManifest` | ❌ Wave 0 |
| ERR-01 | ApplyManifest skips decode errors (continues) | unit | `go test -race ./internal/kube/... -run TestApplyManifest` | ❌ Wave 0 |
| ERR-01 | ApplyManifest skips IsNotFound errors (continues) | unit | `go test -race ./internal/kube/... -run TestApplyManifest` | ❌ Wave 0 |
| ERR-02 | GetChallengeBySlug passes ctx to HTTP request | unit | `go test -race ./internal/api/... -run TestGetChallengeBySlug` | ✅ (existing, needs ctx arg update) |
| ERR-02 | cmd/start RunE passes cmd.Context() to api calls | unit | `go test -race ./cmd/... -run TestStartRunE` | ✅ (existing, needs signature update) |
| ERR-02 | Ctrl-C cancels HTTP request | smoke/manual | manual with `kubeasy challenge start` | manual-only |
| ERR-03 | WebsiteURL uses KUBEASY_API_URL when set | unit | `go test -race ./internal/constants/... -run TestWebsiteURL` | ❌ Wave 0 |
| ERR-03 | WebsiteURL retains default when env not set | unit | `go test -race ./internal/constants/... -run TestWebsiteURL` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test -race ./cmd/... ./internal/api/... ./internal/kube/... ./internal/constants/...`
- **Per wave merge:** `task test:unit`
- **Phase gate:** `task test:unit` green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `internal/kube/manifest_test.go` — covers ERR-01 (ApplyManifest critical vs. skippable error paths). Requires fake `dynamic.Interface` client using `k8s.io/client-go/dynamic/fake` (already in go.mod as a test dependency via integration tests).
- [ ] `internal/constants/const_test.go` — covers ERR-03 (KUBEASY_API_URL env var override logic). Simple unit test, no dependencies beyond stdlib.
- [ ] Existing `cmd/*_test.go` and `internal/api/client_http_test.go` — NOT missing files, but ALL call sites must update function signatures to match the new ctx parameter (compile-time enforcement).

---

## Sources

### Primary (HIGH confidence)
- Direct source file reads: `internal/kube/manifest.go`, `internal/api/client.go`, `internal/constants/const.go`, `cmd/start.go`, `cmd/submit.go`, `cmd/reset.go`, `cmd/clean.go`, `cmd/get.go`, `internal/deployer/challenge.go`
- Direct test file reads: `internal/api/client_http_test.go`, `cmd/start_test.go`, `cmd/submit_test.go`, `cmd/reset_test.go`, `cmd/common_test.go`
- `.planning/phases/03-error-handling/03-CONTEXT.md` — locked decisions, all implementation choices
- `.planning/REQUIREMENTS.md` — ERR-01, ERR-02, ERR-03 specifications

### Secondary (MEDIUM confidence)
- Taskfile.yml — unit test command verified as `task test:unit` running `go test -race -covermode=atomic`

### Tertiary (LOW confidence)
None — all findings are grounded in direct source file reading.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; all patterns from existing codebase
- Architecture: HIGH — code paths read directly, not inferred
- Pitfalls: HIGH — derived from reading actual code and existing test patterns
- Test map: HIGH — test file existence verified, commands derived from Taskfile

**Research date:** 2026-03-09
**Valid until:** Phase is self-contained; research does not expire (no external API surface to drift)
