---
phase: 7
slug: probe-pod-internal-connectivity
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-11
---

# Phase 7 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — existing test setup |
| **Quick run command** | `task test:unit` |
| **Full suite command** | `task test` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `task test:unit`
- **After every plan wave:** Run `task test`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 7-01-01 | 01 | 1 | PROBE-01 | unit | `task test:unit` | ❌ W0 | ⬜ pending |
| 7-01-02 | 01 | 1 | PROBE-01 | unit | `task test:unit` | ❌ W0 | ⬜ pending |
| 7-01-03 | 01 | 1 | PROBE-02 | unit | `task test:unit` | ❌ W0 | ⬜ pending |
| 7-02-01 | 02 | 2 | PROBE-03 | unit | `task test:unit` | ❌ W0 | ⬜ pending |
| 7-02-02 | 02 | 2 | PROBE-04 | unit | `task test:unit` | ❌ W0 | ⬜ pending |
| 7-02-03 | 02 | 2 | CONN-01 | unit | `task test:unit` | ❌ W0 | ⬜ pending |
| 7-02-04 | 02 | 2 | CONN-02 | unit | `task test:unit` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/validation/probe_test.go` — stubs for PROBE-01, PROBE-02, PROBE-03, PROBE-04
- [ ] `internal/validation/executor_connectivity_test.go` — stubs for CONN-01, CONN-02

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Ctrl-C cleanup removes probe pod | PROBE-02 | Requires OS signal interrupt during test run | Start validation, press Ctrl-C, verify pod deleted with `kubectl get pods -n kubeasy-probes` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
