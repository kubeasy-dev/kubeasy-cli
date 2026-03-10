---
phase: 5
slug: security-hardening
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-11
---

# Phase 5 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `testing` stdlib + `testify` (assert/require) |
| **Config file** | none — `go test ./...` convention |
| **Quick run command** | `go test ./internal/validation/... ./internal/kube/... -run "TestCheckConnectivity\|TestFetchManifest" -v` |
| **Full suite command** | `task test:unit` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/validation/... ./internal/kube/... -run "TestCheckConnectivity|TestFetchManifest" -v`
- **After every plan wave:** Run `task test:unit`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** ~5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 5-01-01 | 01 | 1 | SEC-01 | unit | `go test ./internal/validation/... -run TestCheckConnectivity_CurlArgs -v` | Wave 0 | ⬜ pending |
| 5-01-02 | 01 | 1 | SEC-01 | unit | `go test ./internal/validation/... -run TestCheckConnectivity_NoShellFlag -v` | Wave 0 | ⬜ pending |
| 5-01-03 | 01 | 1 | SEC-01 | compile | `go build ./internal/validation/...` | automatic | ⬜ pending |
| 5-02-01 | 02 | 1 | SEC-02 | unit | `go test ./internal/kube/... -run TestFetchManifest_Allowlist -v` | Wave 0 | ⬜ pending |
| 5-02-02 | 02 | 1 | SEC-02 | unit | `go test ./internal/kube/... -run TestFetchManifest_AllowedDomains -v` | Wave 0 | ⬜ pending |
| 5-02-03 | 02 | 1 | SEC-02 | unit | `go test ./internal/kube/... -run TestFetchManifest_RejectedURL_Error -v` | Wave 0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/validation/executor_test.go` — add `TestCheckConnectivity_CurlArgs`, `TestCheckConnectivity_NoShellFlag`, and URL-with-special-chars table test (file exists, new test functions needed)
- [ ] `internal/kube/manifest_test.go` — add `TestFetchManifest_Allowlist` table-driven test (file exists, new test function needed)

*Existing test infrastructure (testify, fake clients) covers all phase requirements. No new framework or fixture files are needed.*

---

## Manual-Only Verifications

*All phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
