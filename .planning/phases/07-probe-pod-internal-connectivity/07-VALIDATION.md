---
phase: 7
slug: probe-pod-internal-connectivity
status: draft
nyquist_compliant: true
wave_0_complete: true
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

Wave 0 test files are created as part of the TDD RED phase in each plan's Task 1. The test stubs are written before implementation, so they exist by the time the GREEN phase runs.

- [ ] `internal/deployer/probe_test.go` — stubs for PROBE-01, PROBE-02, PROBE-03 (created in plan 01 Task 1)
- [ ] `internal/validation/executor_test.go` additions — stubs for PROBE-03, PROBE-04, CONN-01, CONN-02 (created in plan 02 Task 1)
- [ ] `internal/validation/loader_test.go` additions — stub for probe-mode Parse acceptance (created in plan 02 Task 1)

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Ctrl-C cleanup removes probe pod | PROBE-02 | Requires OS signal interrupt during test run | Start validation, press Ctrl-C, verify pod deleted with `kubectl get pods -n kubeasy-probes` |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
