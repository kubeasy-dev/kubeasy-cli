---
phase: 6
slug: infrastructure-foundation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-11
---

# Phase 6 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — existing test infrastructure |
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
| 6-01-01 | 01 | 1 | INFRA-01 | unit | `task test:unit` | ❌ W0 | ⬜ pending |
| 6-01-02 | 01 | 1 | INFRA-02 | unit | `task test:unit` | ❌ W0 | ⬜ pending |
| 6-01-03 | 01 | 1 | INFRA-03 | unit | `task test:unit` | ❌ W0 | ⬜ pending |
| 6-02-01 | 02 | 2 | INFRA-04 | unit | `task test:unit` | ❌ W0 | ⬜ pending |
| 6-02-02 | 02 | 2 | INFRA-05 | unit | `task test:unit` | ❌ W0 | ⬜ pending |
| 6-02-03 | 02 | 2 | INFRA-06 | unit | `task test:unit` | ❌ W0 | ⬜ pending |
| 6-02-04 | 02 | 2 | INFRA-07 | manual | N/A | N/A | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/deployer/infrastructure_test.go` — unit stubs for nginx-ingress, Gateway API, cert-manager install functions
- [ ] `internal/deployer/setup_status_test.go` — unit stubs for per-component status reporting (INFRA-04)

*Existing test infrastructure (go test) covers the framework.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| cloud-provider-kind advisory shown when binary missing | INFRA-07 | Requires binary absence simulation; integration test environment may have it installed | Run `kubeasy setup` without cloud-provider-kind in PATH; verify advisory message shown and setup does not fail |
| GatewayClass registered after setup | INFRA-02 | Requires live Kind cluster with cloud-provider-kind running | Run `kubeasy setup`; verify `kubectl get gatewayclass` shows `cloud-provider-kind` with ACCEPTED=True |
| cert-manager webhook ready after install | INFRA-03 | Requires live cluster with actual TLS bootstrap | Run `kubeasy setup`; verify cert-manager webhook endpoint has addresses; attempt to create a Certificate resource |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
