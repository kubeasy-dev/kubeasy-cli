# Phase 5: Security Hardening - Research

**Researched:** 2026-03-11
**Domain:** Go security — shell injection prevention, HTTP fetch allowlists
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**SEC-01: Shell injection (executeConnectivity)**
- Fix the primary curl path: replace `sh -c "curl ... 'URL'"` with direct args `["curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", "--connect-timeout", "<N>", url]`
- The `escapeShellArg` helper and `sh -c` wrapper are removed from the curl invocation
- Wget fallback: leave as-is for now — revisit in a later phase when the validation approach is re-examined
- SEC-01 is satisfied by fixing the primary (curl) path; the fallback is a deferred improvement

**SEC-02: FetchManifest allowlist**
- Add a domain-level allowlist inside `FetchManifest` — no signature change, no impact on callers
- Trusted domains: `https://github.com/` and `https://raw.githubusercontent.com/`
- Pattern mirrors `loadFromURL` in `loader.go` (validates against `ChallengesRepoBaseURL` before fetching)
- If URL does not match any allowed prefix: return an error, do not fetch
- Remove the `#nosec G107` comment once the allowlist is in place

### Claude's Discretion
- Exact error message format for rejected URLs
- Whether to extract the allowlist into a package-level variable or keep it inline
- Test structure for the allowlist validation

### Deferred Ideas (OUT OF SCOPE)
- Wget fallback shell injection fix — user requested to revisit the connectivity validation approach in a later phase rather than patch the fallback now
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| SEC-01 | `executeConnectivity` uses `exec.Command` with individual arguments instead of `sh -c` to prevent shell injection | Direct `exec.Command` arg passing is the standard Go pattern; `corev1.PodExecOptions.Command` already accepts a `[]string`, so only the slice construction changes |
| SEC-02 | `FetchManifest` is made non-exported or accepts a trusted URL allowlist to prevent arbitrary remote fetches | `loadFromURL` in `loader.go` establishes the exact pattern: `strings.HasPrefix(url, trustedBase)` before `http.Get`; both GitHub domains already cover all current callers |
</phase_requirements>

---

## Summary

Phase 5 fixes two concrete, self-contained attack surfaces. Both fixes are mechanical — they
alter a handful of lines in two files and require no new dependencies, no architectural changes,
and no caller updates.

**SEC-01** (`executeConnectivity` in `internal/validation/executor.go`): The current code
constructs a shell command string `sh -c "curl ... '$URL'"` and relies on `escapeShellArg` to
neutralise embedded quotes. This approach is fragile; bypasses exist and the escaping logic is
custom code. The fix replaces the `sh -c` invocation with a direct `[]string` of arguments
passed to `corev1.PodExecOptions.Command`. Because the Kubernetes exec API routes the slice
directly to the container process without a shell, there is no injection surface regardless of
what the URL contains. The `escapeShellArg` helper becomes dead code and is deleted.

**SEC-02** (`FetchManifest` in `internal/kube/manifest.go`): The function currently performs
an unconstrained `http.Get(url)` with a suppression comment `#nosec G107`. The fix adds a
prefix check against a two-entry allowlist (`https://github.com/` and
`https://raw.githubusercontent.com/`) before the HTTP call. This mirrors exactly the pattern
already used in `loadFromURL` in `loader.go`. Both current callers in `infrastructure.go`
(Kyverno release and local-path-provisioner) produce URLs that start with one of these
prefixes, so runtime behaviour is unchanged.

**Primary recommendation:** Apply both fixes in a single commit — they are independent of each
other and small enough that a combined change is safe and auditable.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `k8s.io/client-go/kubernetes` | existing | Kubernetes exec via SPDY | Already used; `PodExecOptions.Command` is the correct injection point |
| `strings` stdlib | stdlib | `strings.HasPrefix` URL prefix check | Zero dependency; consistent with `loadFromURL` pattern already in codebase |

No new dependencies are required for either fix.

---

## Architecture Patterns

### SEC-01: Replace `sh -c` with direct arg slice

**What:** `corev1.PodExecOptions.Command` accepts a `[]string`. When the slice contains more
than one element (`["curl", "-s", ...]`), the Kubernetes API server passes the arguments
directly to the container's entrypoint via the container runtime — no shell is invoked.

**When to use:** Always when constructing exec commands in Kubernetes. Only use `sh -c` when
shell features (pipes, redirects, glob expansion) are genuinely required and no alternative
exists.

**Current (vulnerable) pattern:**
```go
// internal/validation/executor.go — BEFORE (lines 476-479)
escapedURL := escapeShellArg(target.URL)
cmd := []string{
    "sh", "-c",
    fmt.Sprintf("curl -s -o /dev/null -w '%%{http_code}' --connect-timeout %d '%s'", timeout, escapedURL),
}
```

**Fixed pattern:**
```go
// internal/validation/executor.go — AFTER
cmd := []string{
    "curl", "-s", "-o", "/dev/null",
    "-w", "%{http_code}",
    "--connect-timeout", strconv.Itoa(timeout),
    target.URL,
}
```

Key points:
- `target.URL` is the last positional argument — curl interprets it literally, the kernel
  never passes it through a shell
- `strconv.Itoa(timeout)` converts the integer timeout — `timeout` is already `int` in the
  struct field, so no format string is needed
- The `-w '%{http_code}'` format string no longer needs shell quoting; it is passed directly
- `escapeShellArg` (lines 457-463) becomes unreferenced and must be deleted to satisfy
  `golangci-lint` unused-function checks (the same atomicity constraint seen in Phase 4)

### SEC-02: Domain allowlist in `FetchManifest`

**What:** Before calling `http.Get(url)`, validate that `url` starts with one of the trusted
domain prefixes. Return an error immediately if it does not.

**Established pattern (from `loadFromURL` in `loader.go`, lines 69-76):**
```go
func loadFromURL(url string) (*ValidationConfig, error) {
    if !strings.HasPrefix(url, ChallengesRepoBaseURL) {
        return nil, fmt.Errorf("invalid URL: must be from %s", ChallengesRepoBaseURL)
    }
    resp, err := http.Get(url) //nolint:gosec // URL validated against ChallengesRepoBaseURL
    // ...
}
```

**Fixed pattern for `FetchManifest`:**
```go
// Package-level variable (Claude's discretion — recommended for testability)
var fetchManifestAllowedPrefixes = []string{
    "https://github.com/",
    "https://raw.githubusercontent.com/",
}

func FetchManifest(url string) ([]byte, error) {
    allowed := false
    for _, prefix := range fetchManifestAllowedPrefixes {
        if strings.HasPrefix(url, prefix) {
            allowed = true
            break
        }
    }
    if !allowed {
        return nil, fmt.Errorf("FetchManifest: URL %q is not from a trusted domain", url)
    }
    resp, err := http.Get(url) //nolint:gosec // URL validated against allowlist
    // ... rest unchanged
}
```

Recommendation: extract to a package-level `var` slice (not a constant, since slices cannot
be constants in Go). This makes test injection straightforward — tests can verify allowlist
rejection without making real HTTP calls, by passing URLs that do/don't start with allowed
prefixes.

### Anti-Patterns to Avoid

- **Shell quoting as a security layer:** `escapeShellArg` is defence-in-depth at best.
  Single-quote escaping is bypassable through encoding tricks; the correct fix is to remove
  the shell entirely.
- **`#nosec` without mitigation:** The existing `#nosec G107` comment suppresses the linter
  but does not fix the underlying issue. After the allowlist is in place the comment becomes a
  truthful suppression (`URL validated against allowlist`).
- **Path-level allowlist instead of domain-level:** Checking specific path prefixes (e.g.
  `https://github.com/kyverno/...`) would require allowlist updates for every new manifest
  source. Domain-level is the right scope for this use case.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| URL allowlist | Custom regex or URL parser | `strings.HasPrefix` on trusted domain strings | Already established as the project's pattern; regex adds unnecessary complexity for prefix matching |
| Shell arg escaping | `escapeShellArg` custom helper | Direct `[]string` to `exec.Command` / `PodExecOptions.Command` | Shell removal is always better than shell escaping |

---

## Common Pitfalls

### Pitfall 1: Leaving `escapeShellArg` in place with no callers

**What goes wrong:** After the curl command is rewritten, `escapeShellArg` has no callers.
`golangci-lint` will fail the build with an "unused function" error.

**Why it happens:** Phase 4 established the pattern: when alias functions were removed,
callers had to be updated atomically or the lint check would fail.

**How to avoid:** Delete `escapeShellArg` in the same commit that rewrites `checkConnectivity`.

**Warning signs:** `task lint` output containing `unused` or `deadcode`.

### Pitfall 2: wget fallback still uses `sh -c` — this is intentional

**What goes wrong:** A reviewer notices the wget fallback (lines 506-519) still constructs
`sh -c` and may flag it as an incomplete fix.

**Why it happens:** The user has explicitly deferred the wget fallback to a future phase.

**How to avoid:** Add a `// TODO(sec): wget fallback still uses sh -c — deferred to future phase`
comment on the wget block so future maintainers understand it is a known issue, not an
oversight.

### Pitfall 3: `strconv.Itoa` vs `fmt.Sprintf` for timeout argument

**What goes wrong:** Using `fmt.Sprintf("%d", timeout)` works but is unnecessary. The field
`ConnectivityCheck.TimeoutSeconds` is `int` (confirmed from `types.go`) — `strconv.Itoa`
is the idiomatic conversion.

**How to avoid:** Use `strconv.Itoa(timeout)`. `strconv` is already imported in `executor.go`
(line 7: `"strconv"`).

### Pitfall 4: `FetchManifest` callers use `https://github.com/` URLs

**What goes wrong:** Kyverno URLs are `https://github.com/kyverno/kyverno/releases/download/...`
which starts with `https://github.com/` — confirmed. local-path-provisioner uses
`https://raw.githubusercontent.com/rancher/...` — also confirmed. Both are within the allowlist.

**How to avoid:** Verify both URL-generating functions (`kyvernoInstallURL` and
`localPathProvisionerInstallURL` in `infrastructure.go`) against the allowlist before landing
the change. Both are confirmed safe.

---

## Code Examples

### Verified: current `checkConnectivity` curl command (executor.go lines 473-479)

```go
// Source: internal/validation/executor.go
escapedURL := escapeShellArg(target.URL)
cmd := []string{
    "sh", "-c",
    fmt.Sprintf("curl -s -o /dev/null -w '%%{http_code}' --connect-timeout %d '%s'", timeout, escapedURL),
}
```

### Verified: `loadFromURL` allowlist pattern (loader.go lines 69-76)

```go
// Source: internal/validation/loader.go
func loadFromURL(url string) (*ValidationConfig, error) {
    if !strings.HasPrefix(url, ChallengesRepoBaseURL) {
        return nil, fmt.Errorf("invalid URL: must be from %s", ChallengesRepoBaseURL)
    }
    resp, err := http.Get(url) //nolint:gosec // URL validated against ChallengesRepoBaseURL
```

### Verified: current FetchManifest (manifest.go lines 21-34)

```go
// Source: internal/kube/manifest.go
func FetchManifest(url string) ([]byte, error) {
    resp, err := http.Get(url) // #nosec G107 -- URL is controlled by trusted sources in this context
```

### Verified: Caller URLs (infrastructure.go lines 24-30)

```go
// https://github.com/ prefix — covered by allowlist
func kyvernoInstallURL() string {
    return fmt.Sprintf("https://github.com/kyverno/kyverno/releases/download/%s/install.yaml", KyvernoVersion)
}
// https://raw.githubusercontent.com/ prefix — covered by allowlist
func localPathProvisionerInstallURL() string {
    return fmt.Sprintf("https://raw.githubusercontent.com/rancher/local-path-provisioner/%s/deploy/local-path-storage.yaml", LocalPathProvisionerVersion)
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Shell command string + escaping for K8s exec | Direct `[]string` arg list in `PodExecOptions.Command` | This phase | No shell interpreter involved; injection impossible at the argument boundary |
| Unconstrained `http.Get` with `#nosec` | Allowlist-guarded `http.Get` with truthful suppression comment | This phase | Arbitrary URL fetch becomes a compile-time-auditable error path |

---

## Open Questions

1. **`%{http_code}` format string without shell quoting**
   - What we know: curl accepts `-w %{http_code}` without quotes when passed as a direct arg
   - What's unclear: Whether any container images ship a curl version old enough to require
     different flag syntax
   - Recommendation: Use `%{http_code}` without quoting (shell was the only reason quoting
     was needed); this is the documented curl format string syntax

2. **`nolint` directive placement after allowlist**
   - What we know: `loadFromURL` uses `//nolint:gosec // URL validated against ChallengesRepoBaseURL`
   - Recommendation: Use the same pattern in `FetchManifest`: `//nolint:gosec // URL validated against allowlist`
     This is more descriptive than `#nosec G107` and consistent with the rest of the codebase

---

## Validation Architecture

> `workflow.nyquist_validation` is `true` in `.planning/config.json` — this section is required.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | `testing` stdlib + `testify` (assert/require) |
| Config file | none — `go test ./...` convention |
| Quick run command | `go test ./internal/validation/... ./internal/kube/... -run TestSec -v` |
| Full suite command | `task test:unit` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SEC-01 | `checkConnectivity` curl cmd slice starts with `"curl"`, not `"sh"` | unit | `go test ./internal/validation/... -run TestCheckConnectivity_CurlArgs -v` | Wave 0 |
| SEC-01 | `checkConnectivity` curl cmd does not contain `"-c"` as any element | unit | `go test ./internal/validation/... -run TestCheckConnectivity_NoShellFlag -v` | Wave 0 |
| SEC-01 | `escapeShellArg` is removed (compilation gate) | compile | `go build ./internal/validation/...` | automatic |
| SEC-02 | `FetchManifest` rejects URLs not starting with allowed prefixes | unit | `go test ./internal/kube/... -run TestFetchManifest_Allowlist -v` | Wave 0 |
| SEC-02 | `FetchManifest` accepts both `https://github.com/` and `https://raw.githubusercontent.com/` prefixes (no HTTP call) | unit | `go test ./internal/kube/... -run TestFetchManifest_AllowedDomains -v` | Wave 0 |
| SEC-02 | `FetchManifest` returns descriptive error for blocked URLs | unit | `go test ./internal/kube/... -run TestFetchManifest_RejectedURL_Error -v` | Wave 0 |

### Test Design Notes

**SEC-01 — testing `checkConnectivity` command construction**

`checkConnectivity` is a private method on `*Executor` and currently reaches the network via
`remotecommand.NewSPDYExecutor`. The existing test file (`executor_test.go`) already mocks
`clientset` with `fake.NewClientset()` and does not test the connectivity path (it relies on
SPDY which cannot be faked without a real API server).

The practical approach: extract the command construction into a small pure function that
returns `[]string`, test that function directly.

```go
// Suggested testable helper (Claude's discretion on exact shape)
func buildCurlCommand(url string, timeoutSeconds int) []string {
    return []string{
        "curl", "-s", "-o", "/dev/null",
        "-w", "%{http_code}",
        "--connect-timeout", strconv.Itoa(timeoutSeconds),
        url,
    }
}
```

Tests then assert on the returned slice:
- `cmd[0] == "curl"` (not `"sh"`)
- `!slices.Contains(cmd, "-c")` (no shell flag)
- `cmd[len(cmd)-1] == url` (URL is last, positional, unquoted)
- A URL containing single quotes, spaces, and backticks appears verbatim in the slice

**SEC-02 — testing `FetchManifest` without HTTP calls**

The allowlist check returns before `http.Get` for rejected URLs, so no HTTP mock is needed
for the rejection path. Use table-driven tests:

```go
func TestFetchManifest_Allowlist(t *testing.T) {
    tests := []struct {
        name    string
        url     string
        wantErr bool
        errMsg  string
    }{
        {"github.com allowed", "https://github.com/owner/repo/releases/download/v1/install.yaml", false, ""},
        {"raw.githubusercontent.com allowed", "https://raw.githubusercontent.com/owner/repo/main/file.yaml", false, ""},
        {"blocked domain", "https://evil.example.com/manifest.yaml", true, "not from a trusted domain"},
        {"http downgrade blocked", "http://github.com/owner/repo/file.yaml", true, "not from a trusted domain"},
        {"empty string blocked", "", true, "not from a trusted domain"},
        {"github.com subdomain blocked", "https://api.github.com/repos/...", true, "not from a trusted domain"},
    }
```

For the `wantErr: false` cases the test will reach `http.Get` and fail with a network error
in unit test context. Either:
- Accept the error (test only asserts it is NOT the allowlist error)
- Or use `httptest.NewServer` to serve a fake response

The simplest approach: for allowed URLs, assert the error message does NOT contain "not from
a trusted domain" (the allowlist rejection message). The HTTP connection failure is a separate
concern from the allowlist gate being tested.

### Sampling Rate

- **Per task commit:** `go test ./internal/validation/... ./internal/kube/... -run "TestCheckConnectivity|TestFetchManifest" -v`
- **Per wave merge:** `task test:unit`
- **Phase gate:** `task test:unit` green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `internal/validation/executor_test.go` — add `TestCheckConnectivity_CurlArgs`, `TestCheckConnectivity_NoShellFlag`, and URL-with-special-chars table test (file exists, new test functions needed)
- [ ] `internal/kube/manifest_test.go` — add `TestFetchManifest_Allowlist` table-driven test (file exists, new test function needed)

None — existing test infrastructure (testify, fake clients) covers all phase requirements. No
new framework or fixture files are needed.

---

## Sources

### Primary (HIGH confidence)

- Direct code reading: `internal/validation/executor.go` — lines 457-479 (escapeShellArg and checkConnectivity)
- Direct code reading: `internal/kube/manifest.go` — lines 20-34 (FetchManifest)
- Direct code reading: `internal/validation/loader.go` — lines 68-91 (loadFromURL pattern)
- Direct code reading: `internal/deployer/infrastructure.go` — lines 23-30 (caller URLs)
- Direct code reading: `internal/kube/manifest_test.go` — existing test structure
- Direct code reading: `internal/validation/executor_test.go` — existing test structure

### Secondary (MEDIUM confidence)

- Go standard library `os/exec` documentation: `exec.Command` and `exec.Cmd.Args` — args are
  passed directly to the kernel `execve` syscall; no shell interpretation occurs
- Kubernetes client-go: `corev1.PodExecOptions.Command []string` — documented as a list of
  arguments; the API server passes them verbatim to the container runtime

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; both fixes use stdlib only
- Architecture: HIGH — direct code inspection, pattern already exists in `loadFromURL`
- Pitfalls: HIGH — lint atomicity constraint confirmed from Phase 4 history; caller URLs
  confirmed by reading `infrastructure.go`

**Research date:** 2026-03-11
**Valid until:** Stable — no external dependency changes involved
