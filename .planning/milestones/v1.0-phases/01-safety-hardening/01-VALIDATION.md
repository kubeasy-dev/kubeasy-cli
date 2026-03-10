---
phase: 1
slug: safety-hardening
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-09
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` + `github.com/stretchr/testify` v1.11.1 |
| **Config file** | none (standard `go test`) |
| **Quick run command** | `go test ./internal/validation/... -run 'TestGetGVRForKind\|TestFindLocalChallengeFile\|TestExecute_Malformed' -v` |
| **Full suite command** | `task test:unit` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/validation/... -run 'TestGetGVRForKind\|TestFindLocalChallengeFile\|TestExecute_Malformed' -v`
- **After every plan wave:** Run `task test:unit`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 1-01-01 | 01 | 0 | SAFE-01 | unit | `go test ./internal/validation/... -run TestExecute_MalformedSpec -v` | ❌ Wave 0 | ⬜ pending |
| 1-01-02 | 01 | 0 | SAFE-02 | unit | `go test ./cmd/... -run TestValidateChallengeSlug -v` | ❌ Wave 0 | ⬜ pending |
| 1-01-03 | 01 | 0 | SAFE-03 | unit | `go test ./internal/validation/... -run TestFindLocalChallengeFile_NoHardcodedPath -v` | ❌ Wave 0 | ⬜ pending |
| 1-01-04 | 01 | 0 | TST-04 | unit | `go test ./internal/validation/... -run TestGetGVRForKind -v` | ❌ Wave 0 | ⬜ pending |
| 1-01-05 | 01 | 0 | TST-05 | unit | `go test ./internal/validation/... -run TestFindLocalChallengeFile_NoHardcodedPath -v` | ❌ Wave 0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/validation/executor_test.go` — add `TestExecute_MalformedSpec` stubs for SAFE-01
- [ ] `internal/validation/executor_test.go` — add `TestGetGVRForKind_UnsupportedKind` and `TestGetGVRForKind_SupportedKinds` for TST-04
- [ ] `internal/validation/loader_test.go` — add `TestFindLocalChallengeFile_NoHardcodedPath` and `TestFindLocalChallengeFile_HonorsEnvVar` for SAFE-03 / TST-05
- [ ] `cmd/common_test.go` (new file) — add `TestValidateChallengeSlug` for SAFE-02

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
