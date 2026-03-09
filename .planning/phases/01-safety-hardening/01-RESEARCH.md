# Phase 1: Safety Hardening - Research

**Researched:** 2026-03-09
**Domain:** Go CLI safety — panic prevention, slug validation, build-time path isolation
**Confidence:** HIGH

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| SAFE-01 | All type assertions `v.Spec.(XxxSpec)` in `executor.go` use comma-ok form and return a `Result` with `Passed: false` instead of panicking | Exact location identified in `Execute()` switch block (lines 73-86); comma-ok pattern is standard Go; no library needed |
| SAFE-02 | `validateChallengeSlug` is called at the beginning of `RunE` in `start`, `submit`, `reset`, and `clean` before any API or cluster call | `validateChallengeSlug` already exists in `cmd/common.go`; gap analysis per command documented below |
| SAFE-03 | The hardcoded path `~/Workspace/kubeasy/challenges/` is removed from `loader.go`; local development uses an env var or explicit flag | Path is the third entry in `FindLocalChallengeFile`'s `paths` slice (line 52); env var `KUBEASY_LOCAL_CHALLENGES` pattern identified |
| TST-04 | Unit tests verify that `getGVRForKind` returns a clear error for unsupported kinds without panicking | `getGVRForKind` already returns `error` for the default case; test file exists (`executor_test.go`) but no test for this function yet |
| TST-05 | Unit tests verify that `FindLocalChallengeFile` does not resolve the developer hardcoded path in production builds | `loader_test.go` exists with `TestFindLocalChallengeFile` but only tests local-dir discovery; no test for the hardcoded path absence |
</phase_requirements>

---

## Summary

Phase 1 addresses five specific, well-scoped changes to `internal/validation/executor.go`, `internal/validation/loader.go`, and four command files in `cmd/`. All changes are surgical — no new packages, no new dependencies, no architectural shifts. The brownfield constraint from STATE.md applies: fix implementations only.

The central risk in this codebase is that `executor.go`'s `Execute()` method performs five bare type assertions (`v.Spec.(StatusSpec)`, etc.) that will panic if a `Validation` struct is constructed with a mismatched `Spec` type. Since the loader validates specs at parse time, this is unlikely in production — but a single fuzzing test, a unit test using a zero-value struct, or a future refactor can trigger it. The fix is mechanical: replace each bare assertion with comma-ok.

Slug validation is partially in place. `reset.go` calls `getChallenge()` which already calls `validateChallengeSlug`. `start.go`, `submit.go`, and `clean.go` do not: they call `api.GetChallenge` directly or call `deleteChallengeResources` (which goes straight to `kube.GetKubernetesClient`). The fix is to add a single `validateChallengeSlug(challengeSlug)` call at the top of each `RunE`, before any other work.

The hardcoded path (`~/Workspace/kubeasy/challenges/`) is the third entry in the `paths` slice in `FindLocalChallengeFile`. The fix is to remove that literal and replace it with an optional env var lookup (`KUBEASY_LOCAL_CHALLENGES_DIR`), keeping the first two relative paths for developer convenience in a checked-out monorepo, while ensuring a production binary cannot accidentally load local files from a developer machine.

**Primary recommendation:** All five requirements are purely mechanical code changes with no external dependencies. Tests use the existing `testify` + `k8s.io/client-go/kubernetes/fake` + `k8s.io/client-go/dynamic/fake` infrastructure already present in `executor_test.go` and `loader_test.go`.

---

## Standard Stack

### Core (already in go.mod — no new dependencies needed)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/stretchr/testify` | v1.11.1 | Test assertions (`assert`, `require`) | Already used in all test files |
| `k8s.io/client-go/kubernetes/fake` | v0.35.0 | Mock Kubernetes clientset for unit tests | Used in `executor_test.go` |
| `k8s.io/client-go/dynamic/fake` | v0.35.0 | Mock dynamic client for unit tests | Used in `executor_test.go` |
| `k8s.io/apimachinery/pkg/runtime` | v0.35.0 | `runtime.NewScheme()` for fake clients | Used in `executor_test.go` |

No new `go.mod` entries are required for this phase.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `KUBEASY_LOCAL_CHALLENGES_DIR` env var | `--local-challenges` flag | Flag requires plumbing through Cobra all the way to `LoadForChallenge`; env var is simpler and convention in CLI tools |
| Removing hardcoded path entirely | Build tags to exclude it | Build tags add complexity and a second code path; env var achieves the same goal with less mechanism |

---

## Architecture Patterns

### Gap Analysis: Where slug validation is missing

| Command | Current behavior | Gap |
|---------|-----------------|-----|
| `reset.go` | Calls `getChallenge(slug)` which calls `validateChallengeSlug` | **Already compliant** — no change needed |
| `start.go` | Calls `api.GetChallenge(slug)` directly (no validation first) | Add `validateChallengeSlug(challengeSlug)` at line 22, before `ui.WaitMessage` |
| `submit.go` | Calls `api.GetChallenge(slug)` directly (no validation first) | Add `validateChallengeSlug(challengeSlug)` at line 22, before `ui.WaitMessage` |
| `clean.go` | Calls `deleteChallengeResources(ctx, slug)` directly (no API or validation) | Add `validateChallengeSlug(challengeSlug)` at line 16, before `deleteChallengeResources` |

### Pattern 1: Comma-ok type assertion with Result return

**What:** Replace bare type assertion `spec := v.Spec.(XxxSpec)` with comma-ok form. On assertion failure, return a `Result` with `Passed: false` and a descriptive message.

**When to use:** Everywhere a `Validation.Spec` is cast to a concrete type inside the `Execute()` switch.

**Example:**
```go
// BEFORE — panics if Spec is nil or wrong type
case TypeStatus:
    spec := v.Spec.(StatusSpec)
    result.Passed, result.Message, err = e.executeStatus(ctx, spec)

// AFTER — returns safe error result
case TypeStatus:
    spec, ok := v.Spec.(StatusSpec)
    if !ok {
        result.Message = fmt.Sprintf("internal error: expected StatusSpec, got %T", v.Spec)
        result.Duration = time.Since(start)
        return result
    }
    result.Passed, result.Message, err = e.executeStatus(ctx, spec)
```

All five cases in the switch require the same treatment: `TypeStatus`, `TypeCondition`, `TypeLog`, `TypeEvent`, `TypeConnectivity`.

### Pattern 2: Early slug validation in RunE

**What:** Call `validateChallengeSlug` as the very first statement in `RunE`, before any UI output or external calls.

**When to use:** Any command that accepts a `[challenge-slug]` argument.

**Example:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    challengeSlug := args[0]

    // SAFE-02: validate slug before any API or cluster call
    if err := validateChallengeSlug(challengeSlug); err != nil {
        return err
    }

    ui.Section(fmt.Sprintf("Starting Challenge: %s", challengeSlug))
    // ... rest of RunE
```

Note: `ui.Section` call can remain before the API calls; it does not make any external call. The requirement says "before any API or cluster call", so placing `validateChallengeSlug` before `api.GetChallenge` and `kube.*` calls satisfies it.

### Pattern 3: Env var for local dev path in FindLocalChallengeFile

**What:** Replace the hardcoded `filepath.Join(os.Getenv("HOME"), "Workspace", "kubeasy", "challenges", slug, "challenge.yaml")` with a lookup of `KUBEASY_LOCAL_CHALLENGES_DIR`. If the env var is set, prepend `filepath.Join(os.Getenv("KUBEASY_LOCAL_CHALLENGES_DIR"), slug, "challenge.yaml")` to the search paths. If not set, only the two relative paths remain.

**When to use:** Production binary has the env var unset → hardcoded path is never checked. Developer sets `KUBEASY_LOCAL_CHALLENGES_DIR=~/Workspace/kubeasy/challenges` → same behavior as today.

**Example:**
```go
func FindLocalChallengeFile(slug string) string {
    slug = filepath.Base(slug)

    paths := []string{
        filepath.Join(".", slug, "challenge.yaml"),
        filepath.Join("..", "challenges", slug, "challenge.yaml"),
    }

    // Developer override: set KUBEASY_LOCAL_CHALLENGES_DIR to enable local loading
    if localDir := os.Getenv("KUBEASY_LOCAL_CHALLENGES_DIR"); localDir != "" {
        paths = append(paths, filepath.Join(localDir, slug, "challenge.yaml"))
    }

    for _, p := range paths {
        if _, err := os.Stat(p); err == nil {
            return p
        }
    }
    return ""
}
```

### Anti-Patterns to Avoid

- **Adding `recover()` to `Execute()`:** Using `recover` to catch panics would hide bugs. The comma-ok fix prevents the panic at its source.
- **Removing relative paths from `FindLocalChallengeFile`:** The `./slug/challenge.yaml` and `../challenges/slug/challenge.yaml` paths are legitimate for developers working in a monorepo checkout. Only the absolute `~/Workspace` path is developer-machine-specific and must go.
- **Returning `error` from `Execute()`:** The function signature is `Execute(ctx, Validation) Result`. Changing the signature would require updating all callers. The comma-ok pattern keeps the signature stable.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Fake Kubernetes clients | Custom mock structs | `k8s.io/client-go/kubernetes/fake` + `dynamicfake` | Already in the module; supports full API including list/get/watch |
| Slug validation regex | New regex utility | Existing `validateChallengeSlug` in `cmd/common.go` | Function already exists and is tested-compatible |
| Test fixtures for malformed specs | Manual struct construction | Directly construct `Validation{Key: "x", Type: TypeStatus, Spec: "wrong-type"}` | No fixture library needed — the type system is the fixture |

---

## Common Pitfalls

### Pitfall 1: Forgetting the `start` command skips `getChallenge`

**What goes wrong:** Developer looks at `reset.go`, sees it uses `getChallenge()` (which validates the slug), concludes `start.go` is also safe. But `start.go` calls `api.GetChallenge` directly — bypassing `validateChallengeSlug` entirely.

**Why it happens:** `reset.go` was refactored at some point to use the `getChallenge` helper; `start.go` and `submit.go` were not.

**How to avoid:** Read each RunE individually. The test for SAFE-02 (TST-04 is actually for `getGVRForKind`; the slug test is part of SAFE-02's success criterion) must cover all four commands.

### Pitfall 2: `clean.go` makes NO API call — a malformed slug creates a bad namespace name

**What goes wrong:** `clean.go` calls `deleteChallengeResources` which calls `kube.GetKubernetesClient` and then `deployer.CleanupChallenge`. A slug with uppercase letters or spaces becomes a namespace name passed directly to `clientset.CoreV1().Namespaces().Delete(...)`. Kubernetes will reject it, but the error message is confusing ("namespace not found" rather than "invalid slug").

**How to avoid:** `validateChallengeSlug` call in `clean.go` before `deleteChallengeResources`.

### Pitfall 3: Test for TST-05 must unset `KUBEASY_LOCAL_CHALLENGES_DIR`

**What goes wrong:** If a developer has `KUBEASY_LOCAL_CHALLENGES_DIR` set in their shell, the test that verifies the hardcoded path is absent will pass for the wrong reason (env var resolves instead), or fail unexpectedly.

**How to avoid:** In the test, call `t.Setenv("KUBEASY_LOCAL_CHALLENGES_DIR", "")` to guarantee the env var is cleared for the duration of the test.

### Pitfall 4: Comma-ok on a nil `Spec` returns `false, ok` where ok is false

**What goes wrong:** If a `Validation` has `Spec: nil`, the comma-ok assertion `spec, ok := v.Spec.(StatusSpec)` returns `ok = false`. The error message `fmt.Sprintf("internal error: expected StatusSpec, got %T", v.Spec)` will print `got <nil>`, which is acceptable and informative.

**How to avoid:** No extra nil check needed — the comma-ok pattern already handles this gracefully.

---

## Code Examples

### TST-04: Test for getGVRForKind with unsupported kind

```go
// In internal/validation/executor_test.go
func TestGetGVRForKind_UnsupportedKind(t *testing.T) {
    _, err := getGVRForKind("CronJob") // not in the switch
    require.Error(t, err)
    assert.Contains(t, err.Error(), "unsupported resource kind")
}

func TestGetGVRForKind_SupportedKinds(t *testing.T) {
    kinds := []string{"deployment", "statefulset", "daemonset", "replicaset", "job", "pod", "service"}
    for _, kind := range kinds {
        t.Run(kind, func(t *testing.T) {
            gvr, err := getGVRForKind(kind)
            require.NoError(t, err)
            assert.NotEmpty(t, gvr.Resource)
        })
    }
}
```

Note: `getGVRForKind` is package-private (lowercase). Tests in the same package (`package validation`) can access it directly.

### TST-05: Test that FindLocalChallengeFile ignores hardcoded path in production

```go
// In internal/validation/loader_test.go
func TestFindLocalChallengeFile_NoHardcodedPath(t *testing.T) {
    // Ensure env var is absent (simulates production)
    t.Setenv("KUBEASY_LOCAL_CHALLENGES_DIR", "")

    // Use a slug that would only match via the hardcoded ~/Workspace path
    // (we don't create any temp files, so relative paths won't match either)
    found := FindLocalChallengeFile("nonexistent-challenge-xyz")
    assert.Empty(t, found, "should not find file via hardcoded developer path when env var is unset")
}

func TestFindLocalChallengeFile_HonorsEnvVar(t *testing.T) {
    tmpDir := t.TempDir()
    t.Setenv("KUBEASY_LOCAL_CHALLENGES_DIR", tmpDir)

    // Create challenge.yaml in the temp dir
    challengeDir := filepath.Join(tmpDir, "my-challenge")
    require.NoError(t, os.MkdirAll(challengeDir, 0755))
    require.NoError(t, os.WriteFile(filepath.Join(challengeDir, "challenge.yaml"), []byte("objectives: []"), 0600))

    found := FindLocalChallengeFile("my-challenge")
    assert.NotEmpty(t, found)
    assert.Contains(t, found, "my-challenge/challenge.yaml")
}
```

### SAFE-01: Comma-ok assertion pattern for Execute()

The complete `Execute()` switch after the fix — all five type assertions use comma-ok:

```go
switch v.Type {
case TypeStatus:
    spec, ok := v.Spec.(StatusSpec)
    if !ok {
        result.Message = fmt.Sprintf("internal error: expected StatusSpec, got %T", v.Spec)
        result.Duration = time.Since(start)
        return result
    }
    result.Passed, result.Message, err = e.executeStatus(ctx, spec)
case TypeCondition:
    spec, ok := v.Spec.(ConditionSpec)
    if !ok {
        result.Message = fmt.Sprintf("internal error: expected ConditionSpec, got %T", v.Spec)
        result.Duration = time.Since(start)
        return result
    }
    result.Passed, result.Message, err = e.executeCondition(ctx, spec)
// ... same pattern for TypeLog, TypeEvent, TypeConnectivity
```

---

## State of the Art

| Old Approach | Current Approach | Impact |
|--------------|-----------------|--------|
| Bare type assertion `x := iface.(T)` | Comma-ok `x, ok := iface.(T)` | Prevents panic; standard Go practice since Go 1.0 |
| Hardcoded developer paths in production code | Env var override for dev paths | Separates developer environment from production binary behavior |

---

## Open Questions

1. **Should `cmd/start.go` keep the `ui.Section` call before or after slug validation?**
   - What we know: `validateChallengeSlug` returns immediately without side effects; `ui.Section` only prints to stdout.
   - What's unclear: Whether outputting a section header before detecting an invalid slug is acceptable UX.
   - Recommendation: Place `validateChallengeSlug` before `ui.Section` for strictness (requirement says "before any API or cluster call", not "before any UI call"). Either order satisfies the requirement; before `ui.Section` is cleaner.

2. **Should the env var name be `KUBEASY_LOCAL_CHALLENGES_DIR` or `KUBEASY_CHALLENGES_PATH`?**
   - What we know: No convention is established in the codebase yet. `KUBEASY_API_URL` (planned in ERR-03) uses the `KUBEASY_` prefix.
   - Recommendation: `KUBEASY_LOCAL_CHALLENGES_DIR` — the word `LOCAL` makes intent clear; `DIR` matches the path semantics.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` + `github.com/stretchr/testify` v1.11.1 |
| Config file | none (standard `go test`) |
| Quick run command | `go test ./internal/validation/... -run 'TestGetGVRForKind\|TestFindLocalChallengeFile' -v` |
| Full suite command | `task test:unit` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SAFE-01 | `Execute()` with wrong Spec type returns Result{Passed:false} without panic | unit | `go test ./internal/validation/... -run TestExecute_MalformedSpec -v` | ❌ Wave 0 |
| SAFE-02 | `start`, `submit`, `clean` reject invalid slugs before API call | unit | `go test ./cmd/... -run TestValidateChallengeSlug -v` | ❌ Wave 0 |
| SAFE-03 | `FindLocalChallengeFile` ignores hardcoded path when env var unset | unit | `go test ./internal/validation/... -run TestFindLocalChallengeFile_NoHardcodedPath -v` | ❌ Wave 0 |
| TST-04 | `getGVRForKind` returns error for unsupported kinds | unit | `go test ./internal/validation/... -run TestGetGVRForKind -v` | ❌ Wave 0 |
| TST-05 | `FindLocalChallengeFile` does not resolve developer hardcoded path in production | unit | `go test ./internal/validation/... -run TestFindLocalChallengeFile_NoHardcodedPath -v` | ❌ Wave 0 |

Note: SAFE-02 command tests (`cmd/` package) require care because Cobra `RunE` functions depend on `cmd.Context()` and mocked API clients. The slug validation itself (`validateChallengeSlug`) is already testable in isolation from `cmd/common.go` — test it directly rather than through the full command pipeline.

### Sampling Rate

- **Per task commit:** `go test ./internal/validation/... -run 'TestGetGVRForKind\|TestFindLocalChallengeFile\|TestExecute_Malformed' -v`
- **Per wave merge:** `task test:unit`
- **Phase gate:** `task test:unit` green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] Tests for `getGVRForKind` in `internal/validation/executor_test.go` — covers TST-04
- [ ] Tests for `FindLocalChallengeFile` env var behavior in `internal/validation/loader_test.go` — covers TST-05 and SAFE-03 verification
- [ ] Tests for malformed Spec assertions in `internal/validation/executor_test.go` — covers SAFE-01
- [ ] `validateChallengeSlug` direct unit test in `cmd/common_test.go` (new file) — covers SAFE-02 indirectly

---

## Sources

### Primary (HIGH confidence)

- Direct source code read of `internal/validation/executor.go` — identified 5 bare type assertions at lines 73-86
- Direct source code read of `internal/validation/loader.go` — identified hardcoded path at line 52 and `FindLocalChallengeFile` structure
- Direct source code read of `cmd/start.go`, `cmd/submit.go`, `cmd/reset.go`, `cmd/clean.go` — confirmed which commands call `validateChallengeSlug` and which bypass it
- Direct source code read of `cmd/common.go` — confirmed `validateChallengeSlug` and `getChallenge` exist with correct logic
- Direct source code read of `internal/validation/executor_test.go`, `loader_test.go` — confirmed test infrastructure and identified missing test cases
- `go.mod` — confirmed `testify v1.11.1`, `k8s.io/client-go v0.35.0` with fake clients available
- `Taskfile.yml` — confirmed `task test:unit` as the canonical test command

### Secondary (MEDIUM confidence)

- Go specification on type assertions — comma-ok form is guaranteed to not panic (https://go.dev/ref/spec#Type_assertions)

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; all libraries are in go.mod and actively used
- Architecture: HIGH — changes are surgical and directly derived from reading the source files
- Pitfalls: HIGH — identified by reading all four command RunE functions and tracing execution paths

**Research date:** 2026-03-09
**Valid until:** 2026-06-09 (stable Go codebase; no fast-moving dependencies)
