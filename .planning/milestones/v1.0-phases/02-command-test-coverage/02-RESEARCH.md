# Phase 2: Command Test Coverage - Research

**Researched:** 2026-03-09
**Domain:** Go unit testing with Cobra — function-variable mocking for cmd/ layer
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Scope reduit — guards de flux uniquement**
- Test only orchestration logic in `cmd/` not covered by `internal/` tests
- `start.go`: if progress already "in_progress" or "completed" → returns nil without deploying
- `submit.go`: if progress nil → return nil; if progress "completed" → return nil
- `reset.go` and `clean.go`: error propagation from slug validation
- No Kubernetes/deployer mocking — those packages are covered by their own tests

**Mocking strategy — function variables**
- Use function variables in `cmd/` to mock `api.*` in tests
- Pattern: `var apiGetChallenge = api.GetChallenge` — replaced in tests by a fake
- Minimal refactoring, no interface changes

**Alignment reset.go**
- Add `validateChallengeSlug` first in `reset.go` before `getChallenge()`
- Consistent behavior with `clean.go`: invalid slug → immediate error without API call

**ui.* in tests**
- Let `ui.Section`, `ui.WaitMessage`, etc. execute normally in tests
- Output appears in test logs with `-v` — no stdout redirection

### Claude's Discretion
- Exact naming of function variables (e.g., `apiGetChallenge` vs `getChallengeFn`)
- Organization of test files (one file per command or single `commands_test.go`)
- Exact values of fakes (test slugs, API response structures)

### Deferred Ideas (OUT OF SCOPE)
- Mocking Kubernetes/deployer for full flow testing — too much refactoring for the value, internal packages already covered
- Interface injection for commands — cleaner long-term, but scope creep for this phase
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| TST-01 | Unit tests cover `RunE` of `cmd/start.go` (slug validation, progress state, API call sequence) | Function-variable pattern enables testing all three branches without a real API |
| TST-02 | Unit tests cover `RunE` of `cmd/submit.go` (validation loading, execution, result submission) | Same pattern; submit has two early-exit guards testable without Kubernetes |
| TST-03 | Unit tests cover `RunE` of `cmd/reset.go` and `cmd/clean.go` including error paths | `reset.go` needs `validateChallengeSlug` added first; `clean.go` already has it |
</phase_requirements>

## Summary

Phase 2 adds unit tests to the four command `RunE` functions in `cmd/` that have zero test coverage today. The scope is narrow: test the flow-guard logic — early exits based on progress state, slug validation errors, and simulated API failures. Kubernetes/deployer paths are explicitly out of scope.

The mocking strategy is function variables: each `api.*` call site is fronted by a `var` of the same signature, initialized to the real function. Tests replace these vars with fakes. This is idiomatic Go when you need lightweight mocking without interfaces, and it requires that tests run in `package cmd` (not `package cmd_test`) to access unexported vars — which the existing `common_test.go` already does.

One production change is needed: `reset.go` currently calls `getChallenge()` (which internally calls `validateChallengeSlug`) but does not call `validateChallengeSlug` at the top of `RunE` itself. Adding it as the first statement aligns `reset.go` with the other three commands.

**Primary recommendation:** Declare function variables adjacent to each command file, use `t.Cleanup` to restore them after each test, and invoke `RunE` directly (not via `cobra.Command.Execute`) to capture the error return cleanly.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `testing` | stdlib | Test framework | Built-in Go |
| `github.com/stretchr/testify/assert` | v1.11.1 | Non-fatal assertions | Already in go.mod; used in existing tests |
| `github.com/stretchr/testify/require` | v1.11.1 | Fatal assertions (fail fast) | Already in go.mod; used in existing tests |
| `github.com/spf13/cobra` | v1.10.2 | Command invocation in tests | Already in go.mod; used by the commands |

### No New Dependencies
All required libraries are already present in `go.mod`. This phase adds zero new dependencies.

**Installation:** Nothing to install.

## Architecture Patterns

### Recommended File Organization

The planner has discretion here. Two options exist; the recommended approach is one file per command for clarity and parallel plan execution:

```
cmd/
├── common.go                # validateChallengeSlug, getChallenge, deleteChallengeResources
├── common_test.go           # existing: TestValidateChallengeSlug
├── start.go                 # function vars declared here or in start_test.go
├── start_test.go            # TestStartRunE_*
├── submit.go
├── submit_test.go           # TestSubmitRunE_*
├── reset.go                 # needs validateChallengeSlug added at top of RunE
├── reset_test.go            # TestResetRunE_*
├── clean.go
└── clean_test.go            # TestCleanRunE_*
```

Alternatively, all new tests in a single `commands_test.go` also works — the planner decides.

### Pattern 1: Function Variable Injection

**What:** Replace direct `api.Foo(...)` calls with a package-level `var` of the same signature. In production the var holds the real function. In tests, replace it with a fake.

**When to use:** When the callee is a package-level function (not a method on an interface), and creating a full interface would require changing multiple call sites.

**Example (in `start.go`):**
```go
// Source: established Go pattern — no external library required
var (
    apiGetChallenge        = api.GetChallenge
    apiGetChallengeProgress = api.GetChallengeProgress
    apiStartChallenge      = func(slug string) error { return api.StartChallenge(slug) }
)
```

**Example (in `start_test.go`):**
```go
// Source: package cmd — same package as cmd, can access unexported vars
func TestStartRunE_AlreadyInProgress(t *testing.T) {
    orig := apiGetChallenge
    t.Cleanup(func() { apiGetChallenge = orig })

    apiGetChallenge = func(slug string) (*api.ChallengeEntity, error) {
        return &api.ChallengeEntity{Title: "Test"}, nil
    }
    origProgress := apiGetChallengeProgress
    t.Cleanup(func() { apiGetChallengeProgress = origProgress })
    apiGetChallengeProgress = func(slug string) (*api.ChallengeStatusResponse, error) {
        return &api.ChallengeStatusResponse{Status: "in_progress"}, nil
    }

    err := startChallengeCmd.RunE(startChallengeCmd, []string{"pod-evicted"})
    assert.NoError(t, err) // early exit, returns nil
}
```

### Pattern 2: Direct RunE Invocation (not cobra.Execute)

**What:** Call `cmd.RunE(cmd, args)` directly instead of `rootCmd.Execute()`. This bypasses `PersistentPreRun` (logger init, CI mode) and returns the error directly without `os.Exit`.

**When to use:** In all cmd unit tests. Using `rootCmd.Execute()` would trigger `os.Exit(1)` on error and prevent the test from capturing the error return.

**Critical detail:** `PersistentPreRun` in `root.go` calls `logger.Initialize` and `ui.SetCIMode`. When invoking `RunE` directly, `PersistentPreRun` does NOT run. This means:
- `ui.WaitMessage` will use spinner mode (pterm), not CI text mode
- The logger will not be initialized for the test run
- This is acceptable per CONTEXT.md decision: "let ui.* execute normally in tests"

**Example:**
```go
err := cleanChallengeCmd.RunE(cleanChallengeCmd, []string{"pod-evicted"})
require.Error(t, err)
```

### Pattern 3: Table-Driven Tests with t.Run

Already established in the project. Follow the same pattern as `TestValidateChallengeSlug` in `common_test.go`.

```go
func TestCleanRunE_SlugValidation(t *testing.T) {
    tests := []struct {
        name    string
        slug    string
        wantErr bool
    }{
        {name: "invalid_slug", slug: "INVALID", wantErr: true},
        {name: "valid_slug_clean_succeeds", slug: "pod-evicted", wantErr: false},
    }
    for _, tc := range tests {
        tc := tc
        t.Run(tc.name, func(t *testing.T) {
            // set up fakes, invoke RunE, assert
        })
    }
}
```

### Anti-Patterns to Avoid

- **Using `rootCmd.Execute()` in tests:** Calls `os.Exit(1)` on error; the test process dies instead of failing cleanly.
- **Mutating function vars without `t.Cleanup`:** If a test panics before manual restore, subsequent tests see stale fakes. Always use `t.Cleanup`.
- **Testing Kubernetes paths:** `kube.GetKubernetesClient()` and `deployer.DeployChallenge()` fail immediately without a real cluster. The locked decision caps tests at the API call sequence, before those calls.
- **Re-testing `validateChallengeSlug` logic:** Already covered in `common_test.go`. The new tests need only one "invalid slug" case per command to verify propagation, not exhaustive slug format cases.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Fake HTTP server | `httptest.NewServer` with route handlers | `apiGetChallenge` function var fake | Function var is 3 lines; httptest requires auth, TLS, keyring setup — far outside scope |
| Interface-based mock | Generate mock with `mockery` or `gomock` | Function var reassignment | No interface exists, and CONTEXT.md explicitly locks this as deferred |
| Capturing stdout | `os.Pipe` redirect | Let pterm write to stdout | CONTEXT.md decision: no stdout redirection |

**Key insight:** The function-variable pattern is precisely scoped for this brownfield codebase. It requires no new tooling and no interface extraction.

## Common Pitfalls

### Pitfall 1: Running Tests Without a Kind Cluster Causes Panic in Deep Paths
**What goes wrong:** If a test reaches `kube.GetKubernetesClient()`, it fails to find the `kind-kubeasy` context and returns an error; that error propagates correctly. But if `deleteChallengeResources` is reached and `kube.GetKubernetesClient()` returns an error, `clean.go` already handles it with `return fmt.Errorf(...)`. No panic, but the test would be testing the wrong thing.
**Why it happens:** `clean.go` and `reset.go` call `deleteChallengeResources` after slug validation. If fakes are not set up for the Kubernetes path, the test falls through to cluster calls.
**How to avoid:** For `clean.go` and `reset.go`, the scope is slug-validation-only. Only test the `validateChallengeSlug` error path (invalid slug → immediate return). Do not attempt to mock past the cluster calls.
**Warning signs:** Test duration > 1s; any log message about kubeconfig.

### Pitfall 2: `t.Parallel()` with Shared Package-Level Vars
**What goes wrong:** Function vars are package-level globals. If tests run in parallel and mutate the same var, they race.
**Why it happens:** `go test -race` (used in `task test:unit`) will detect and fail on data races.
**How to avoid:** Do not call `t.Parallel()` on tests that mutate function vars, OR serialize them with subtests that avoid parallelism. The simplest choice: omit `t.Parallel()` for tests using function var injection.

### Pitfall 3: `reset.go` Missing `validateChallengeSlug` at Top of RunE
**What goes wrong:** Without the change, `reset.go` calls `getChallenge(slug)` which internally calls `validateChallengeSlug`, so slug validation does happen — but it goes through an API call wrapper. TST-03 requires testing the slug-validation error path. If `validateChallengeSlug` is not the first statement, tests would need to also mock `api.GetChallenge` just to reach the validation error, which is awkward.
**Why it happens:** `reset.go` was not updated in Phase 1 (SAFE-02 only touched `start`, `submit`, `clean`).
**How to avoid:** Add `if err := validateChallengeSlug(challengeSlug); err != nil { return err }` as the first statement in `reset.go`'s `RunE`, before the `ui.Section` call.

### Pitfall 4: `submit.go` Tests That Reach `kube.GetKubernetesClient()`
**What goes wrong:** `submit.go` calls `kube.GetKubernetesClient()` after checking progress. If a test provides a non-nil, non-completed progress fake, execution reaches the Kubernetes call and fails.
**Why it happens:** The submit scope in CONTEXT.md only covers guards before the K8s path: nil progress → return nil, completed → return nil.
**How to avoid:** The three testable paths for submit are: (1) invalid slug, (2) nil progress, (3) completed progress. All three return before reaching `kube.GetKubernetesClient()`. Do not attempt paths beyond those.

### Pitfall 5: ui.WaitMessage Spinner in Non-TTY Test Environment
**What goes wrong:** `pterm.DefaultSpinner.Start()` in a non-TTY environment (CI, `go test` piped output) may produce garbled output but does not error. Tests pass regardless.
**Why it happens:** `PersistentPreRun` sets `ui.SetCIMode(true)` when stdout is not a TTY, but `RunE` is called directly without `PersistentPreRun` running.
**How to avoid:** Either call `ui.SetCIMode(true)` in a `TestMain` for the `cmd` package, or accept the spinner output in test logs. CONTEXT.md locks the decision as "let ui.* execute normally." Adding `TestMain` that sets CI mode is an option at the planner's discretion — it cleans up test output but is not required.

## Code Examples

### Anatomy of a start.go Guard Test

```go
// Source: established function-var pattern; matches cmd/common_test.go style
func TestStartRunE_AlreadyCompleted(t *testing.T) {
    // Save and restore originals
    origGetChallenge := apiGetChallenge
    t.Cleanup(func() { apiGetChallenge = origGetChallenge })
    origGetProgress := apiGetChallengeProgress
    t.Cleanup(func() { apiGetChallengeProgress = origGetProgress })

    apiGetChallenge = func(slug string) (*api.ChallengeEntity, error) {
        return &api.ChallengeEntity{Title: "Test Challenge"}, nil
    }
    apiGetChallengeProgress = func(slug string) (*api.ChallengeStatusResponse, error) {
        return &api.ChallengeStatusResponse{Status: "completed"}, nil
    }

    err := startChallengeCmd.RunE(startChallengeCmd, []string{"pod-evicted"})
    assert.NoError(t, err) // guard returns nil
}
```

### Simulated API Failure

```go
// Source: established function-var pattern
func TestStartRunE_APIFailure(t *testing.T) {
    orig := apiGetChallenge
    t.Cleanup(func() { apiGetChallenge = orig })

    apiGetChallenge = func(slug string) (*api.ChallengeEntity, error) {
        return nil, fmt.Errorf("network error")
    }

    err := startChallengeCmd.RunE(startChallengeCmd, []string{"pod-evicted"})
    require.Error(t, err)                       // must not be nil
    assert.NotPanics(t, func() {               // must not panic
        _ = startChallengeCmd.RunE(startChallengeCmd, []string{"pod-evicted"})
    })
}
```

### Invalid Slug — Direct Return Without Any Mock

```go
// Source: established function-var pattern; no fakes needed for slug validation
func TestCleanRunE_InvalidSlug(t *testing.T) {
    err := cleanChallengeCmd.RunE(cleanChallengeCmd, []string{"INVALID_SLUG"})
    require.Error(t, err)
    assert.Contains(t, err.Error(), "invalid challenge slug")
}
```

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + testify v1.11.1 |
| Config file | none (go test flags in Taskfile.yml) |
| Quick run command | `go test ./cmd/... -run TestStartRunE -v` |
| Full suite command | `task test:unit` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TST-01 | `start.go` invalid slug → error | unit | `go test ./cmd/... -run TestStartRunE_InvalidSlug` | Wave 0 |
| TST-01 | `start.go` progress "in_progress" → nil | unit | `go test ./cmd/... -run TestStartRunE_AlreadyInProgress` | Wave 0 |
| TST-01 | `start.go` progress "completed" → nil | unit | `go test ./cmd/... -run TestStartRunE_AlreadyCompleted` | Wave 0 |
| TST-01 | `start.go` API failure → error, no panic | unit | `go test ./cmd/... -run TestStartRunE_APIFailure` | Wave 0 |
| TST-02 | `submit.go` invalid slug → error | unit | `go test ./cmd/... -run TestSubmitRunE_InvalidSlug` | Wave 0 |
| TST-02 | `submit.go` progress nil → nil | unit | `go test ./cmd/... -run TestSubmitRunE_ProgressNil` | Wave 0 |
| TST-02 | `submit.go` progress "completed" → nil | unit | `go test ./cmd/... -run TestSubmitRunE_AlreadyCompleted` | Wave 0 |
| TST-02 | `submit.go` API failure → error, no panic | unit | `go test ./cmd/... -run TestSubmitRunE_APIFailure` | Wave 0 |
| TST-03 | `reset.go` invalid slug → error | unit | `go test ./cmd/... -run TestResetRunE_InvalidSlug` | Wave 0 |
| TST-03 | `reset.go` API failure → error, no panic | unit | `go test ./cmd/... -run TestResetRunE_APIFailure` | Wave 0 |
| TST-03 | `clean.go` invalid slug → error | unit | `go test ./cmd/... -run TestCleanRunE_InvalidSlug` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./cmd/... -v -race`
- **Per wave merge:** `task test:unit`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `cmd/start_test.go` — covers TST-01 (4 test cases)
- [ ] `cmd/submit_test.go` — covers TST-02 (4 test cases)
- [ ] `cmd/reset_test.go` — covers TST-03 reset cases (2 test cases)
- [ ] `cmd/clean_test.go` — covers TST-03 clean cases (1 test case)
- [ ] Production change in `cmd/reset.go` — add `validateChallengeSlug` as first RunE statement

Note: No new framework setup needed. `testify` is already a dependency.

## Implementation Details by Command

### start.go — Function Variables Needed

Current direct calls that need var-fronting:
1. `api.GetChallenge(challengeSlug)` — line 34
2. `api.GetChallengeProgress(challengeSlug)` — line 49
3. `api.StartChallenge(challengeSlug)` — line 106

Testable guards (all before Kubernetes calls):
- Invalid slug → `validateChallengeSlug` returns error (no vars needed)
- `apiGetChallenge` returns error → `RunE` returns wrapped error
- `apiGetChallengeProgress` returns error → `RunE` returns wrapped error
- Progress is "in_progress" or "completed" → `RunE` returns nil

### submit.go — Function Variables Needed

Current direct calls that need var-fronting:
1. `api.GetChallenge(challengeSlug)` — line 32 (within WaitMessage)
2. `api.GetChallengeProgress(challengeSlug)` — line 44 (within WaitMessage)

Testable guards (all before Kubernetes calls at line 82+):
- Invalid slug → error
- `apiGetChallenge` returns error → wrapped error
- `apiGetChallengeProgress` returns error → wrapped error
- Progress nil → `RunE` returns nil (line 52-55)
- Progress "completed" → `RunE` returns nil (line 58-61)

### reset.go — Production Change Required + Function Variables Needed

**Production change:** Add at top of RunE (before `ui.Section`):
```go
if err := validateChallengeSlug(challengeSlug); err != nil {
    return err
}
```

Current direct calls that need var-fronting:
1. `api.GetChallenge(slug)` — inside `getChallenge()` in common.go
2. `api.ResetChallengeProgress(challengeSlug)` — line 35

Note: `getChallenge` in `common.go` already calls `validateChallengeSlug` internally. After adding the upfront check in `reset.go`, the invalid-slug test needs no API mock.

Testable guards:
- Invalid slug → error (after production change, no mocks needed)
- `apiGetChallenge` returns error → `getChallenge` returns error → `RunE` returns error
- `apiResetChallengeProgress` returns error → wrapped error

### clean.go — Function Variables Needed

`clean.go` calls `deleteChallengeResources` (which calls `kube.GetKubernetesClient`) after slug validation. The only testable path without cluster mocking is the invalid slug path. For the valid slug path, the test would hit `kube.GetKubernetesClient()` and fail — which is out of scope per CONTEXT.md.

Testable guard:
- Invalid slug → error (no mocks needed)

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Direct `api.Foo()` calls | Function var `var apiFoo = api.Foo` | Phase 2 (this phase) | Tests can replace fakes without interface extraction |
| No slug check in reset.go | `validateChallengeSlug` first in RunE | Phase 2 (this phase) | Consistent with start/submit/clean; enables clean test without API mock |

## Open Questions

1. **`cmd/` package coverage in `task test:unit`**
   - What we know: `test:unit` uses `-coverpkg=./internal/...` — cmd tests RUN but do not appear in coverage percentage
   - What's unclear: Whether the planner should also update `coverpkg` to include `./cmd/...`
   - Recommendation: Out of scope for Phase 2. Tests run and pass; coverage display is cosmetic. A future QUAL phase could update Taskfile.

2. **`common.go` function vars placement**
   - What we know: `getChallenge()` in `common.go` calls `api.GetChallenge` directly. For reset tests, this call would need to be mockable.
   - What's unclear: Whether the var for `api.GetChallenge` should live in `common.go` (shared) or separately in each command file
   - Recommendation: Declare vars in the file that uses them. If both `start.go` and `reset.go` need `apiGetChallenge`, declare it once in `common.go`. If `getChallenge()` itself needs to be replaceable for `reset.go` tests, add `var getChallengeFn = getChallenge` in `reset.go` — planner decides naming per CONTEXT.md discretion.

## Sources

### Primary (HIGH confidence)
- Direct code reading: `cmd/start.go`, `cmd/submit.go`, `cmd/reset.go`, `cmd/clean.go`, `cmd/common.go` — exact function signatures, call sites, flow guards
- Direct code reading: `cmd/common_test.go` — established test pattern (package cmd, testify, table-driven)
- Direct code reading: `internal/api/client.go`, `internal/api/types.go` — exact function signatures for var declarations
- Direct code reading: `internal/ui/ui.go` — WaitMessage behavior, CI mode detection
- Direct code reading: `Taskfile.yml` — `task test:unit` command definition and coverpkg scope

### Secondary (MEDIUM confidence)
- Go documentation on function values: https://go.dev/ref/spec#Function_types — function variables as mock injection pattern is standard Go
- Cobra documentation: RunE returns error directly; Execute() calls os.Exit — verified by reading root.go behavior

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; all from go.mod
- Architecture: HIGH — verified against actual source code
- Pitfalls: HIGH — derived from reading actual implementation

**Research date:** 2026-03-09
**Valid until:** 2026-06-09 (stable — Go stdlib patterns do not change)
