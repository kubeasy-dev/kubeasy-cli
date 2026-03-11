---
phase: 9
slug: tls-validation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-11
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
| 9-01-01 | 01 | 1 | TLS-01, TLS-02, TLS-03 | unit | `task test:unit` | ❌ W0 | ⬜ pending |
| 9-01-02 | 01 | 1 | TLS-01 | unit | `task test:unit` | ❌ W0 | ⬜ pending |
| 9-01-03 | 01 | 1 | TLS-02 | unit | `task test:unit` | ❌ W0 | ⬜ pending |
| 9-01-04 | 01 | 1 | TLS-03 | unit | `task test:unit` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] New test functions in `internal/validation/executor_test.go` — stubs for TLS-01, TLS-02, TLS-03 (file exists, append new test functions)
- [ ] New test functions in `internal/validation/loader_test.go` — stubs for `tls:` YAML parsing (file exists, append)

*No new framework or config needed — existing `go testing` + `testify` infrastructure covers all phase requirements.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `insecureSkipVerify: true` against cert-manager self-signed cert in live Kind cluster | TLS-03 | Requires running Kind cluster with cert-manager | Run `kubeasy challenge start <slug-with-tls>` and submit — check HTTPS succeeds |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
