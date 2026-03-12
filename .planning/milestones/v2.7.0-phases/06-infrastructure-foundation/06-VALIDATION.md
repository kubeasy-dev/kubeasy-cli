---
phase: 6
slug: infrastructure-foundation
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-03-11
audited: 2026-03-11
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
| 6-01-01 | 01 | 1 | INFRA-06 | unit | `task test:unit` | ✅ infrastructure_test.go | ✅ green |
| 6-01-02 | 01 | 1 | INFRA-07 | unit | `task test:unit` | ✅ infrastructure_test.go | ✅ green |
| 6-02-01 | 02 | 2 | INFRA-01 | unit | `task test:unit` | ✅ infrastructure_test.go | ✅ green |
| 6-02-02 | 02 | 2 | INFRA-02 | unit | `task test:unit` | ✅ infrastructure_test.go | ✅ green |
| 6-02-03 | 02 | 2 | INFRA-05 | unit | `task test:unit` | ✅ cloud_provider_kind_test.go | ✅ green |
| 6-03-01 | 03 | 2 | INFRA-03 | unit | `task test:unit` | ✅ infrastructure_test.go | ✅ green |
| 6-03-02 | 03 | 2 | INFRA-04 | unit | `task test:unit` | ✅ infrastructure_test.go | ✅ green |
| 6-04-01 | 04 | 3 | INFRA-07 | manual | N/A | N/A | ✅ manual-verified |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `internal/deployer/infrastructure_test.go` — unit tests for nginx-ingress, Gateway API, cert-manager install functions (31 tests — all passing)
- [x] `internal/deployer/cloud_provider_kind_test.go` — unit tests for cloud-provider-kind binary URL and platform detection (3 tests — all passing)

*Note: Per-component status reporting tests (INFRA-07) are in `infrastructure_test.go` (TestInstallKyverno_AlreadyReady, TestInstallLocalPathProvisioner_AlreadyReady, TestComponentResult_*) rather than a separate `setup_status_test.go` file. Coverage is equivalent.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| cloud-provider-kind advisory shown when binary missing | INFRA-07 | Requires binary absence simulation; integration test environment may have it installed | Run `kubeasy setup` without cloud-provider-kind in PATH; verify advisory message shown and setup does not fail |
| GatewayClass registered after setup | INFRA-02 | Requires live Kind cluster with cloud-provider-kind running | Run `kubeasy setup`; verify `kubectl get gatewayclass` shows `cloud-provider-kind` with ACCEPTED=True |
| cert-manager webhook ready after install | INFRA-03 | Requires live cluster with actual TLS bootstrap | Run `kubeasy setup`; verify cert-manager webhook endpoint has addresses; attempt to create a Certificate resource |
| All 6 components show per-component status lines | INFRA-07 | Requires full live setup run | Human-verified live on 2026-03-11: all 6 components showed "ready" status lines after `kubeasy setup` |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** retroactively signed off 2026-03-11 (all Wave 0 tests written and passing during phase execution; frontmatter not updated at time of commit)

---

## Validation Audit 2026-03-11
| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |
| Tests confirmed passing | 34 (31 infrastructure + 3 cloud_provider_kind) |
