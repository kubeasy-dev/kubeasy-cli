---
phase: 2
slug: command-test-coverage
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-09
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) |
| **Config file** | Taskfile.yml (`test:unit` task) |
| **Quick run command** | `go test ./cmd/... -run TestCmd -v` |
| **Full suite command** | `task test:unit` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./cmd/... -run TestCmd -v`
- **After every plan wave:** Run `task test:unit`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 02-01-01 | 01 | 0 | TST-01 | unit | `go test ./cmd/... -run TestStart -v` | ❌ W0 | ⬜ pending |
| 02-01-02 | 01 | 1 | TST-01 | unit | `go test ./cmd/... -run TestStart -v` | ❌ W0 | ⬜ pending |
| 02-02-01 | 02 | 0 | TST-02 | unit | `go test ./cmd/... -run TestSubmit -v` | ❌ W0 | ⬜ pending |
| 02-02-02 | 02 | 1 | TST-02 | unit | `go test ./cmd/... -run TestSubmit -v` | ❌ W0 | ⬜ pending |
| 02-03-01 | 03 | 0 | TST-03 | unit | `go test ./cmd/... -run TestReset\|TestClean -v` | ❌ W0 | ⬜ pending |
| 02-03-02 | 03 | 1 | TST-03 | unit | `go test ./cmd/... -run TestReset\|TestClean -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `cmd/start_test.go` — stubs for TST-01 (invalid slug, already in_progress, already completed, API failure)
- [ ] `cmd/submit_test.go` — stubs for TST-02 (invalid slug, nil progress, completed progress, API failure)
- [ ] `cmd/reset_test.go` — stubs for TST-03 reset paths (invalid slug, API failure)
- [ ] `cmd/clean_test.go` — stubs for TST-03 clean paths (invalid slug)
- [ ] Function var declarations in each command file or `common.go` for API mocking

*Framework (go test) already present — no installation required.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Full cluster deploy flow | TST-01 | Requires Kind cluster | Run `kubeasy challenge start <slug>` against live cluster |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
