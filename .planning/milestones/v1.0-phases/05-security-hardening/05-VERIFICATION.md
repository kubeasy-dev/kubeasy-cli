---
phase: 05-security-hardening
verified: 2026-03-11T00:00:00Z
status: passed
score: 7/7 must-haves verified
re_verification: false
---

# Phase 5: Security Hardening Verification Report

**Phase Goal:** Fix critical security vulnerabilities — eliminate shell injection in the connectivity validator and add URL allowlisting to the manifest fetcher.
**Verified:** 2026-03-11
**Status:** PASSED
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| #  | Truth                                                                                                       | Status     | Evidence                                                                                                       |
|----|-------------------------------------------------------------------------------------------------------------|------------|----------------------------------------------------------------------------------------------------------------|
| 1  | `executeConnectivity` passes target.URL as a positional argument directly to PodExecOptions.Command — no shell is invoked | VERIFIED | `buildCurlCommand` returns direct `[]string`; `cmd := buildCurlCommand(target.URL, timeout)` at line 476; `Command: cmd` at line 484 |
| 2  | `escapeShellArg` is deleted and the build remains lint-clean                                                | VERIFIED   | grep for `escapeShellArg` in `internal/` returns zero matches                                                 |
| 3  | The curl command slice starts with `curl`, not `sh`                                                         | VERIFIED   | `buildCurlCommand` returns `[]string{"curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", "--connect-timeout", ..., url}`; `TestCheckConnectivity_CurlArgs` and `TestCheckConnectivity_NoShellFlag` pass |
| 4  | `FetchManifest` rejects any URL not starting with `https://github.com/` or `https://raw.githubusercontent.com/` | VERIFIED | `fetchManifestAllowedPrefixes` var + prefix-loop guard in manifest.go lines 22–38; `TestFetchManifest_Allowlist` 7 subtests pass |
| 5  | `FetchManifest` returns a descriptive error for rejected URLs before making any HTTP call                   | VERIFIED   | Guard returns error before `http.Get` is reached; error message contains "not from a trusted domain"          |
| 6  | The `#nosec G107` comment is replaced with a truthful `//nolint:gosec` comment                             | VERIFIED   | `#nosec` absent from manifest.go (grep returns no matches); `//nolint:gosec // URL validated against fetchManifestAllowedPrefixes` present at line 40 |
| 7  | Existing callers in `deployer/infrastructure.go` are unaffected (their URLs already match the allowlist)   | VERIFIED   | Callers produce `https://github.com/kyverno/...` and `https://raw.githubusercontent.com/rancher/...` — both match allowed prefixes; no changes to deployer files |

**Score:** 7/7 truths verified

---

### Required Artifacts

| Artifact                                       | Expected                                               | Status   | Details                                                                                                          |
|------------------------------------------------|--------------------------------------------------------|----------|------------------------------------------------------------------------------------------------------------------|
| `internal/validation/executor.go`              | `buildCurlCommand` helper + rewritten curl block       | VERIFIED | `buildCurlCommand` function at lines 457–466; curl block rewritten at line 476; `escapeShellArg` absent         |
| `internal/validation/executor_test.go`         | Tests for `buildCurlCommand` verifying no shell injection | VERIFIED | 5 tests: `TestCheckConnectivity_CurlArgs`, `TestCheckConnectivity_NoShellFlag`, `TestCheckConnectivity_URLPositional`, `TestCheckConnectivity_SpecialCharsURL`, `TestCheckConnectivity_TimeoutArg` — all pass |
| `internal/kube/manifest.go`                    | `fetchManifestAllowedPrefixes` var + allowlist guard   | VERIFIED | Package-level var at lines 22–25; guard loop at lines 29–38; `//nolint:gosec` at line 40                       |
| `internal/kube/manifest_test.go`               | Table-driven allowlist tests for `FetchManifest`       | VERIFIED | `TestFetchManifest_Allowlist` at line 506 with 6 subtests (4 blocked, 2 allowed) — all 7 runs pass             |

---

### Key Link Verification

| From                                    | To                        | Via                          | Status   | Details                                                                                 |
|-----------------------------------------|---------------------------|------------------------------|----------|-----------------------------------------------------------------------------------------|
| `internal/validation/executor_test.go`  | `buildCurlCommand`        | direct call                  | WIRED    | 5 test functions call `buildCurlCommand(...)` directly at lines 2641, 2651, 2663, 2673, 2681 |
| `checkConnectivity`                     | `PodExecOptions.Command`  | `buildCurlCommand` return value | WIRED | `cmd := buildCurlCommand(target.URL, timeout)` at line 476; `Command: cmd` at line 484  |
| `internal/kube/manifest.go`             | `fetchManifestAllowedPrefixes` | prefix loop before http.Get | WIRED | Loop at lines 30–34 iterates `fetchManifestAllowedPrefixes`; guard at lines 36–38 returns error before `http.Get` at line 40 |
| `internal/kube/manifest_test.go`        | `FetchManifest`           | direct call                  | WIRED    | `FetchManifest(tt.url)` called in table loop at line 546                                |

---

### Requirements Coverage

| Requirement | Source Plan | Description                                                                                          | Status    | Evidence                                                                                     |
|-------------|-------------|------------------------------------------------------------------------------------------------------|-----------|----------------------------------------------------------------------------------------------|
| SEC-01      | 05-01-PLAN  | `executeConnectivity` uses individual arguments instead of `sh -c` to prevent shell injection        | SATISFIED | `buildCurlCommand` returns direct arg slice; no `sh`/`-c` in curl path; `escapeShellArg` deleted; 5 passing tests |
| SEC-02      | 05-02-PLAN  | `FetchManifest` accepts an allowlist of trusted URLs to prevent calls to arbitrary URLs              | SATISFIED | `fetchManifestAllowedPrefixes` var with 2 trusted prefixes; guard rejects before `http.Get`; 7 passing subtests |

No orphaned requirements — all Phase 5 requirement IDs (SEC-01, SEC-02) are accounted for in plan frontmatter and verified in the codebase.

---

### Anti-Patterns Found

| File                                          | Line | Pattern               | Severity | Impact                                                                                     |
|-----------------------------------------------|------|-----------------------|----------|--------------------------------------------------------------------------------------------|
| `internal/validation/executor.go`             | 503  | `TODO(sec)` wget fallback still uses `sh -c` | INFO | wget fallback retains the shell invocation pattern — intentionally deferred; documented with comment |

No blockers or warnings found. The wget `TODO(sec)` is expected per plan scope — it was explicitly deferred and annotated.

---

### Human Verification Required

None. All must-haves are verifiable programmatically:
- Test pass/fail is observable via `go test`
- Function presence/absence is observable via grep
- Code structure (arg slice vs shell string) is observable in source

---

### Gaps Summary

No gaps. All 7 observable truths are verified, all 4 artifacts exist and are substantive and wired, all key links are confirmed, and both requirements (SEC-01, SEC-02) are fully satisfied.

The phase goal is achieved: shell injection in `executeConnectivity` is eliminated (direct arg slice, no `sh -c`, `escapeShellArg` deleted), and `FetchManifest` enforces a domain allowlist before any HTTP call.

---

_Verified: 2026-03-11_
_Verifier: Claude (gsd-verifier)_
