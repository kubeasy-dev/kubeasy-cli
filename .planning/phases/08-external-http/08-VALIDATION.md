---
phase: 8
slug: external-http
status: draft
nyquist_compliant: false
wave_0_complete: false
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
| 08-01-01 | 01 | 0 | EXT-01 | unit | `go test ./internal/validation/... -run TestParse_Connectivity_ExternalMode -v` | ❌ W0 | ⬜ pending |
| 08-01-02 | 01 | 0 | EXT-01 | unit | `go test ./internal/validation/... -run TestParse_Connectivity_ExternalModeWithSourcePod -v` | ❌ W0 | ⬜ pending |
| 08-01-03 | 01 | 0 | EXT-01 | unit | `go test ./internal/validation/... -run TestCheckExternalConnectivity_Success -v` | ❌ W0 | ⬜ pending |
| 08-01-04 | 01 | 0 | EXT-01 | unit | `go test ./internal/validation/... -run TestCheckExternalConnectivity_WrongStatus -v` | ❌ W0 | ⬜ pending |
| 08-01-05 | 01 | 0 | EXT-01 | unit | `go test ./internal/validation/... -run TestCheckExternalConnectivity_BlockedConnection -v` | ❌ W0 | ⬜ pending |
| 08-01-06 | 01 | 0 | EXT-01 | unit | `go test ./internal/validation/... -run TestCheckExternalConnectivity_ConnectionRefused -v` | ❌ W0 | ⬜ pending |
| 08-02-01 | 01 | 0 | EXT-02 | unit | `go test ./internal/validation/... -run TestCheckExternalConnectivity_HostHeader -v` | ❌ W0 | ⬜ pending |
| 08-03-01 | 01 | 0 | EXT-03 | unit | `go test ./internal/validation/... -run TestParse_Connectivity_SslipIO -v` | ❌ W0 | ⬜ pending |
| 08-04-01 | 01 | 0 | EXT-04 | unit | `go test ./internal/validation/... -run TestCheckExternalConnectivity_StatusCodes -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `TestParse_Connectivity_ExternalMode` in `internal/validation/loader_test.go` — stubs for EXT-01 parse path
- [ ] `TestParse_Connectivity_ExternalModeWithSourcePod` in `internal/validation/loader_test.go` — covers EXT-01 error
- [ ] `TestCheckExternalConnectivity_Success` in `internal/validation/executor_test.go` — covers EXT-01, EXT-04 (uses `httptest.NewServer`)
- [ ] `TestCheckExternalConnectivity_WrongStatus` in `internal/validation/executor_test.go` — covers EXT-04
- [ ] `TestCheckExternalConnectivity_BlockedConnection` in `internal/validation/executor_test.go` — covers EXT-01 status-0 guard
- [ ] `TestCheckExternalConnectivity_ConnectionRefused` in `internal/validation/executor_test.go` — covers EXT-01 failure path
- [ ] `TestCheckExternalConnectivity_HostHeader` in `internal/validation/executor_test.go` — covers EXT-02 (assert req.Host via httptest server)
- [ ] `TestParse_Connectivity_SslipIO` in `internal/validation/loader_test.go` — covers EXT-03 (parse only, no DNS call)
- [ ] `TestCheckExternalConnectivity_StatusCodes` in `internal/validation/executor_test.go` — covers EXT-04

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
