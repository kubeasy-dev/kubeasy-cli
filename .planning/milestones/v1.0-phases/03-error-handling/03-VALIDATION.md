---
phase: 3
slug: error-handling
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-09
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test + testify (already in go.mod) |
| **Config file** | none — Go uses `go test` directly |
| **Quick run command** | `go test -race ./cmd/... ./internal/api/... ./internal/kube/... ./internal/constants/...` |
| **Full suite command** | `task test:unit` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -race ./cmd/... ./internal/api/... ./internal/kube/... ./internal/constants/...`
- **After every plan wave:** Run `task test:unit`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 3-01-01 | 01 | 1 | ERR-01 | unit | `go test -race ./internal/kube/... -run TestApplyManifest` | ❌ W0 | ⬜ pending |
| 3-01-02 | 01 | 1 | ERR-01 | unit | `go test -race ./internal/kube/... -run TestApplyManifest` | ❌ W0 | ⬜ pending |
| 3-01-03 | 01 | 1 | ERR-01 | unit | `go test -race ./internal/kube/... -run TestApplyManifest` | ❌ W0 | ⬜ pending |
| 3-02-01 | 02 | 2 | ERR-02 | unit | `go test -race ./internal/api/... -run TestGetChallengeBySlug` | ✅ (needs ctx update) | ⬜ pending |
| 3-02-02 | 02 | 2 | ERR-02 | unit | `go test -race ./cmd/... -run TestStartRunE` | ✅ (needs sig update) | ⬜ pending |
| 3-02-03 | 02 | 2 | ERR-02 | manual | manual with `kubeasy challenge start` + Ctrl-C | manual-only | ⬜ pending |
| 3-03-01 | 03 | 3 | ERR-03 | unit | `go test -race ./internal/constants/... -run TestWebsiteURL` | ❌ W0 | ⬜ pending |
| 3-03-02 | 03 | 3 | ERR-03 | unit | `go test -race ./internal/constants/... -run TestWebsiteURL` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/kube/manifest_test.go` — stubs for ERR-01 (ApplyManifest critical vs. skippable error paths; requires `k8s.io/client-go/dynamic/fake` already in go.mod)
- [ ] `internal/constants/const_test.go` — stubs for ERR-03 (KUBEASY_API_URL env var override logic; stdlib only)
- [ ] Update existing `cmd/*_test.go` and `internal/api/client_http_test.go` anonymous function signatures to accept `ctx context.Context` — compile-time enforcement for ERR-02

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Ctrl-C cancels in-flight HTTP request within 1s | ERR-02 | Requires real network + signal; no test harness for OS signals in unit tests | Run `kubeasy challenge start <slug>`, press Ctrl-C during API call, verify CLI exits within 1s |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
