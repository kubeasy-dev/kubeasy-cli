---
phase: 8
slug: external-http
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-03-11
---

# Phase 8 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify v1.x (existing) |
| **Config file** | Taskfile.yml (`task test:unit`) |
| **Quick run command** | `go test ./internal/validation/... -run TestExternal -v` |
| **Full suite command** | `task test:unit` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/validation/... -count=1`
- **After every plan wave:** Run `task test:unit`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 08-01-01 | 01 | 0 | EXT-01 | unit | `go test ./internal/validation/... -run TestParse_Connectivity_ExternalMode -v` | ✅ W0 | ✅ green |
| 08-01-02 | 01 | 0 | EXT-01 | unit | `go test ./internal/validation/... -run TestParse_Connectivity_ExternalModeWithSourcePod -v` | ✅ W0 | ✅ green |
| 08-01-03 | 01 | 0 | EXT-01 | unit | `go test ./internal/validation/... -run TestCheckExternalConnectivity_Success -v` | ✅ W0 | ✅ green |
| 08-01-04 | 01 | 0 | EXT-01 | unit | `go test ./internal/validation/... -run TestCheckExternalConnectivity_WrongStatus -v` | ✅ W0 | ✅ green |
| 08-01-05 | 01 | 0 | EXT-01 | unit | `go test ./internal/validation/... -run TestCheckExternalConnectivity_BlockedConnection -v` | ✅ W0 | ✅ green |
| 08-01-06 | 01 | 0 | EXT-01 | unit | `go test ./internal/validation/... -run TestCheckExternalConnectivity_ConnectionRefused -v` | ✅ W0 | ✅ green |
| 08-02-01 | 01 | 0 | EXT-02 | unit | `go test ./internal/validation/... -run TestCheckExternalConnectivity_HostHeader -v` | ✅ W0 | ✅ green |
| 08-03-01 | 01 | 0 | EXT-03 | unit | `go test ./internal/validation/... -run TestParse_Connectivity_SslipIO -v` | ✅ W0 | ✅ green |
| 08-04-01 | 01 | 0 | EXT-04 | unit | `go test ./internal/validation/... -run TestCheckExternalConnectivity_StatusCodes -v` | ✅ W0 | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `TestParse_Connectivity_ExternalMode` in `internal/validation/loader_test.go` — stubs for EXT-01 parse path
- [x] `TestParse_Connectivity_ExternalModeWithSourcePod` in `internal/validation/loader_test.go` — covers EXT-01 error
- [x] `TestCheckExternalConnectivity_Success` in `internal/validation/executor_test.go` — covers EXT-01, EXT-04 (uses `httptest.NewServer`)
- [x] `TestCheckExternalConnectivity_WrongStatus` in `internal/validation/executor_test.go` — covers EXT-04
- [x] `TestCheckExternalConnectivity_BlockedConnection` in `internal/validation/executor_test.go` — covers EXT-01 status-0 guard
- [x] `TestCheckExternalConnectivity_ConnectionRefused` in `internal/validation/executor_test.go` — covers EXT-01 failure path
- [x] `TestCheckExternalConnectivity_HostHeader` in `internal/validation/executor_test.go` — covers EXT-02 (assert req.Host via httptest server)
- [x] `TestParse_Connectivity_SslipIO` in `internal/validation/loader_test.go` — covers EXT-03 (parse only, no DNS call)
- [x] `TestCheckExternalConnectivity_StatusCodes` in `internal/validation/executor_test.go` — covers EXT-04

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 10s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** retroactive sign-off 2026-03-11 — all 15 test cases confirmed passing

---

## Validation Audit

**Audited:** 2026-03-11
**Auditor:** gsd-nyquist-auditor (Claude Sonnet 4.6)
**Method:** Retroactive — tests existed from TDD execution; audit confirmed all pass

### Test Execution Results

Command run:
```
go test ./internal/validation/... -run "TestParse_Connectivity_ExternalMode|TestParse_Connectivity_ExternalModeWithSourcePod|TestCheckExternalConnectivity_Success|TestCheckExternalConnectivity_WrongStatus|TestCheckExternalConnectivity_BlockedConnection|TestCheckExternalConnectivity_ConnectionRefused|TestCheckExternalConnectivity_HostHeader|TestParse_Connectivity_SslipIO|TestCheckExternalConnectivity_StatusCodes" -v
```

Result: **15 passed** across 2 packages (loader_test.go + executor_test.go), 0 failures.

### Coverage Map

| Requirement | Test(s) | File | Status |
|-------------|---------|------|--------|
| EXT-01: external mode type field | TestParse_Connectivity_ExternalMode, TestCheckExternalConnectivity_Success/WrongStatus/BlockedConnection/ConnectionRefused | loader_test.go, executor_test.go | green |
| EXT-02: Host header override | TestCheckExternalConnectivity_HostHeader | executor_test.go | green |
| EXT-03: sslip.io URL parsing | TestParse_Connectivity_SslipIO | loader_test.go | green |
| EXT-04: status code matching | TestCheckExternalConnectivity_StatusCodes, TestCheckExternalConnectivity_WrongStatus | executor_test.go | green |

**Conclusion:** All Phase 08 requirements (EXT-01/02/03/04) are fully covered by passing automated tests. No gaps found. Sign-off granted.
