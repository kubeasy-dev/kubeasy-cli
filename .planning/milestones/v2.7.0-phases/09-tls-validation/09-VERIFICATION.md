---
phase: 09-tls-validation
verified: 2026-03-11T00:00:00Z
status: passed
score: 11/11 must-haves verified
re_verification: false
gaps: []
human_verification:
  - test: "Run kubeasy challenge submit against a challenge with a cert-manager self-signed HTTPS endpoint and tls: { insecureSkipVerify: true } in challenge.yaml"
    expected: "Connectivity validation reports passed=true despite self-signed cert"
    why_human: "Requires a live Kind cluster with cert-manager and an active challenge — not exercisable in unit tests"
---

# Phase 9: TLS Validation Verification Report

**Phase Goal:** Add TLS certificate validation support to the connectivity validator so challenge authors can assert TLS properties (certificate validity, expiry, SAN matching) via challenge.yaml.
**Verified:** 2026-03-11
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

All truths are drawn from the combined must_haves of 09-01-PLAN.md and 09-02-PLAN.md.

| #  | Truth | Status | Evidence |
|----|-------|--------|----------|
| 1  | TLSConfig struct exists with InsecureSkipVerify, ValidateExpiry, ValidateSANs bool fields | VERIFIED | `internal/validation/types.go` lines 220-233 |
| 2  | ConnectivityCheck has a TLS *TLSConfig pointer field (nil = no explicit TLS checks) | VERIFIED | `types.go` line 214: `TLS *TLSConfig \`yaml:"tls,omitempty"\`` |
| 3  | challenge.yaml with a tls: block parses without error | VERIFIED | `TestParseConnectivityTLSBlock` in loader_test.go line 851 — 6 sub-tests all PASS |
| 4  | insecureSkipVerify: true parses to TLSConfig.InsecureSkipVerify == true | VERIFIED | loader_test.go line 869 sub-test asserts `InsecureSkipVerify == true` |
| 5  | External check with validateExpiry: true fails with "Certificate expired on YYYY-MM-DD (N days ago)" for an expired cert | VERIFIED | executor_test.go line 3133, executor.go line 535 |
| 6  | External check with validateExpiry: true passes for a valid cert | VERIFIED | executor_test.go line 3154 — asserts no "Certificate expired on" in message |
| 7  | External check with validateSANs: true fails with 'Hostname X not in SANs: [...]' for a hostname not in the cert | VERIFIED | executor_test.go line 3182, executor.go line 542 |
| 8  | External check with validateSANs: true uses HostHeader hostname for SAN matching (not URL host) | VERIFIED | `hostnameForSAN()` helper (executor.go line 627) returns HostHeader when set; Test E uses HostHeader "myapp.other-domain.io" |
| 9  | External check with insecureSkipVerify: true succeeds against a self-signed httptest TLS server | VERIFIED | executor_test.go line 3118 (Test A) asserts passed=true |
| 10 | TLS failure short-circuits the HTTP status check — no "got status 0" message when TLS is the failure | VERIFIED | executor_test.go line 3219 (Test G) asserts msg does NOT contain "got status" |
| 11 | insecureSkipVerify: true takes priority — no cert inspection runs even when validateExpiry or validateSANs also true | VERIFIED | executor_test.go line 3239 (Test H): expired cert + both flags → passed=true, no "Certificate expired on" |

**Score:** 11/11 truths verified

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/validation/types.go` | TLSConfig struct and TLS field on ConnectivityCheck | VERIFIED | Lines 211-233: struct and field present with correct yaml/json tags and godoc |
| `internal/validation/loader_test.go` | 6 sub-tests: tls block parses, insecureSkipVerify, validateExpiry, validateSANs, all-true, empty block | VERIFIED | `TestParseConnectivityTLSBlock` at line 851, all 6 sub-tests present and passing |
| `internal/validation/executor.go` | checkExternalConnectivity with TLS probe, expiry check, SAN check, insecureSkipVerify transport | VERIFIED | Lines 505-588 + helpers `probeTLSCert` (line 593) and `hostnameForSAN` (line 627) |
| `internal/validation/executor_test.go` | 8 TLS test cases covering TLS-01, TLS-02, TLS-03 scenarios using httptest servers | VERIFIED | `TestCheckExternalConnectivityTLS` at line 3110 with 8 named sub-tests |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `executor.go checkExternalConnectivity` | `crypto/tls tls.Dialer.DialContext` | cert probe in `probeTLSCert` | WIRED | `tls.Dialer{...}.DialContext(ctx, "tcp", ...)` at executor.go line 607 |
| `executor.go checkExternalConnectivity` | `crypto/x509 cert.VerifyHostname` | SAN check using HostHeader or URL hostname | WIRED | `cert.VerifyHostname(hostname)` at executor.go line 541 |
| `loader.go TypeConnectivity parse block` | `types.go TLSConfig` | `yaml.Unmarshal` auto-populates via ConnectivityCheck.TLS field | WIRED | No explicit parse code needed — pointer field deserialized automatically; loader.go comment confirms at line ~84 |
| `ConnectivityCheck.TLS` | `checkExternalConnectivity TLS logic` | `target.TLS != nil` guards in executor.go | WIRED | executor.go lines 519-545: three-step guard chain on target.TLS |

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| TLS-01 | 09-01-PLAN.md, 09-02-PLAN.md | External check validates cert NotAfter > now | SATISFIED | `probeTLSCert` + expiry check at executor.go lines 531-536; 2 test cases in executor_test.go |
| TLS-02 | 09-01-PLAN.md, 09-02-PLAN.md | External check validates hostname appears in cert DNSNames | SATISFIED | `cert.VerifyHostname(hostname)` + `hostnameForSAN` at executor.go lines 539-544; 2 test cases in executor_test.go |
| TLS-03 | 09-01-PLAN.md, 09-02-PLAN.md | ConnectivityCheck supports insecureSkipVerify: true for self-signed certs | SATISFIED | `tlsCfg.InsecureSkipVerify = true` at executor.go line 520; Test A confirms self-signed httptest server passes |

All 3 requirements marked Complete in REQUIREMENTS.md, confirmed satisfied with implementation evidence.

---

### Anti-Patterns Found

No anti-patterns detected in phase-modified files.

Scanned: `internal/validation/types.go`, `internal/validation/executor.go`, `internal/validation/loader_test.go`, `internal/validation/executor_test.go`

- No TODO/FIXME/placeholder comments in TLS-related code
- No empty implementations (all TLS handlers return real values)
- No stub handlers (all 8 test scenarios use real httptest servers with actual TLS)
- nolint directives used correctly and only where necessary (gosec G402 on struct literal `InsecureSkipVerify: true` inside `probeTLSCert`)

---

### Human Verification Required

#### 1. End-to-end TLS validation against Kind cluster

**Test:** Start a challenge that deploys an HTTPS service with a cert-manager self-signed certificate. In `challenge.yaml`, add a connectivity check with `tls: { insecureSkipVerify: true }` targeting the HTTPS endpoint. Run `kubeasy challenge submit <slug>`.
**Expected:** The connectivity validation reports `passed: true` despite the self-signed cert not being in the OS trust store.
**Why human:** Requires a live Kind cluster, cert-manager installation, and an active challenge deployment — not exercisable in unit tests. The unit test (Test A) covers the logic path but uses an httptest server, not a real Kind cluster.

---

### Phase Summary

Phase 9 achieves its goal. The TLS validation feature is fully implemented across both plans:

**Plan 01 (types + parsing):** `TLSConfig` struct added to `types.go` with three bool fields and complete godoc. `ConnectivityCheck.TLS *TLSConfig` pointer field wired in with correct yaml/json tags. Six loader tests prove YAML round-trip for all field combinations. No changes to loader.go parse logic were needed — `yaml.Unmarshal` handles the pointer field automatically.

**Plan 02 (executor logic):** `checkExternalConnectivity` extended with a three-step TLS chain: (1) build `tlsCfg` with optional `InsecureSkipVerify`, (2) probe raw cert via `tls.Dialer.DialContext` when `ValidateExpiry` or `ValidateSANs` are requested and `InsecureSkipVerify` is false, then apply manual cert checks with friendly error messages, (3) make HTTP request with the configured transport. Short-circuit on TLS failure is confirmed. `insecureSkipVerify` priority over other checks is confirmed. Two helpers extracted: `probeTLSCert` and `hostnameForSAN`.

All 8 executor TLS test cases pass. All 6 loader TLS test cases pass. Full unit suite passes (0 failures). All 3 requirement IDs (TLS-01, TLS-02, TLS-03) are satisfied and marked Complete in REQUIREMENTS.md.

Commits: `2dfd111` (feat(09-01)), `0b318a4` (test(09-02)), `78e5940` (feat(09-02)) — all verified present in git log.

---

_Verified: 2026-03-11_
_Verifier: Claude (gsd-verifier)_
