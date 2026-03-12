---
phase: 9
slug: tls-validation
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-03-11
signed_off: 2026-03-11
---

# Phase 9 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` + `testify` v1.11.1 |
| **Config file** | none — standard `go test ./...` |
| **Quick run command** | `task test:unit` |
| **Full suite command** | `task test` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `task test:unit`
- **After every plan wave:** Run `task test`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** ~15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 9-01-01 | 01 | 1 | TLS-01, TLS-02, TLS-03 | unit | `task test:unit` | ✅ | ✅ green |
| 9-01-02 | 01 | 1 | TLS-01 | unit | `task test:unit` | ✅ | ✅ green |
| 9-01-03 | 01 | 1 | TLS-02 | unit | `task test:unit` | ✅ | ✅ green |
| 9-01-04 | 01 | 1 | TLS-03 | unit | `task test:unit` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] New test functions in `internal/validation/executor_test.go` — `TestCheckExternalConnectivityTLS` with 8 sub-tests covering TLS-01, TLS-02, TLS-03 (file exists, tests confirmed passing)
- [x] New test functions in `internal/validation/loader_test.go` — `TestParseConnectivityTLSBlock` with 6 sub-tests for `tls:` YAML parsing (file exists, tests confirmed passing)

*No new framework or config needed — existing `go testing` + `testify` infrastructure covers all phase requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `insecureSkipVerify: true` against cert-manager self-signed cert in live Kind cluster | TLS-03 | Requires running Kind cluster with cert-manager | Run `kubeasy challenge start <slug-with-tls>` and submit — check HTTPS succeeds |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 15s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** signed off 2026-03-11

---

## Validation Audit (retroactive, 2026-03-11)

**Auditor:** gsd-nyquist-auditor (Claude Sonnet 4.6)

**Findings:** All Phase 9 requirements (TLS-01/02/03) are covered by existing automated tests. No gaps in test coverage — the only issue was that this VALIDATION.md sign-off checklist had not been ticked after execution.

### Evidence

| Requirement | Test Function | Sub-tests | Command | Result |
|-------------|---------------|-----------|---------|--------|
| TLS-01 (validateExpiry) | `TestCheckExternalConnectivityTLS` | "TLS-01 expired cert fails with friendly message", "TLS-01 valid cert passes expiry check" | `go test -v -run TestCheckExternalConnectivityTLS ./internal/validation/` | PASS |
| TLS-02 (validateSANs) | `TestCheckExternalConnectivityTLS` | "TLS-02 SAN mismatch fails with friendly message", "TLS-02 matching SAN passes check" | `go test -v -run TestCheckExternalConnectivityTLS ./internal/validation/` | PASS |
| TLS-03 (insecureSkipVerify) | `TestCheckExternalConnectivityTLS` | "TLS-03 insecureSkipVerify passes self-signed cert", "insecureSkipVerify takes priority over validateExpiry" | `go test -v -run TestCheckExternalConnectivityTLS ./internal/validation/` | PASS |
| Short-circuit behavior | `TestCheckExternalConnectivityTLS` | "TLS failure short-circuits HTTP status check" | `go test -v -run TestCheckExternalConnectivityTLS ./internal/validation/` | PASS |
| YAML parsing (TLSConfig) | `TestParseConnectivityTLSBlock` | 6 sub-tests: nil-when-absent, each field, all-true, empty-block | `task test:unit` | PASS |

**Total tests confirmed passing:** 8 executor + 6 loader = 14 unit tests
**Full suite status:** `task test:unit` — all packages `ok`, zero failures
