---
phase: 07-probe-pod-internal-connectivity
plan: "01"
subsystem: deployer
tags: [probe-pod, connectivity, tdd, lifecycle]
dependency_graph:
  requires: []
  provides: [CreateProbePod, DeleteProbePod, WaitForProbePodReady, SourcePod.Namespace]
  affects: [internal/validation/executor.go]
tech_stack:
  added: [k8s.io/apimachinery/pkg/util/wait, k8s.io/apimachinery/pkg/api/errors]
  patterns: [fake-clientset-testing, PollUntilContextTimeout, independent-context-cleanup]
key_files:
  created:
    - internal/deployer/probe.go
    - internal/deployer/probe_test.go
  modified:
    - internal/deployer/const.go
    - internal/validation/types.go
decisions:
  - "DeleteProbePod uses context.Background()+10s internally (not caller ctx) to guarantee cleanup on cancellation (PROBE-03)"
  - "Pre-commit hook enforces build passing — TDD RED commit merged with GREEN into single commit"
metrics:
  duration: "201s (~3.5min)"
  completed_date: "2026-03-11"
  tasks_completed: 3
  files_modified: 4
---

# Phase 07 Plan 01: Probe Pod Deployer Summary

**One-liner:** CLI-managed curlimages/curl probe pod lifecycle (create/delete/wait) with PROBE-03 independent-context cleanup contract and SourcePod.Namespace field for cross-namespace connectivity.

## What Was Built

Three exported functions in `internal/deployer/probe.go` manage the kubeasy-probe pod lifecycle:

- **`CreateProbePod(ctx, clientset, namespace)`** — deletes any stale pod first (via `deleteProbePodWithCtx`), polls until gone, then creates a fresh pod with fixed labels `{app: kubeasy-probe, managed-by: kubeasy}`, `RestartPolicy: Never`, container `curl` using `curlimages/curl:VERSION`, and resource requests cpu=10m/memory=16Mi.
- **`DeleteProbePod(ctx, clientset, namespace)`** — deletes with `GracePeriodSeconds=0`, treats NotFound as success. Uses `context.Background()+10s` internally to satisfy the PROBE-03 independent-context cleanup contract.
- **`WaitForProbePodReady(ctx, clientset, namespace)`** — polls with `wait.PollUntilContextTimeout` at 1s interval until pod reaches `Running` phase or ctx deadline exceeded.

Constants and helpers added to `internal/deployer/const.go`:
- `ProbePodName = "kubeasy-probe"` (fixed name for stable NetworkPolicy targeting)
- `ProbePodImageVersion = "8.13.0"` (Renovate-managed via `datasource=docker depName=curlimages/curl`)
- `probePodImage()` private helper

`SourcePod.Namespace string` field added to `internal/validation/types.go` (yaml/json `omitempty` tagged).

## Tasks Completed

| Task | Description | Commit |
|------|-------------|--------|
| 1 | Write failing tests (RED) | 7e18d11 |
| 2 | Implement probe deployer (GREEN) | 7e18d11 |
| 3 | Full unit suite verification | 7e18d11 |

Note: Tasks 1 and 2 share a single commit because the pre-commit hook enforces lint + build passing — the RED state (undefined symbols) cannot be committed separately. The TDD flow was followed in implementation order.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Pre-commit hook prevents RED-state commit**
- **Found during:** Task 1 commit attempt
- **Issue:** The Husky pre-commit hook runs golangci-lint which requires the package to compile. The RED state (undefined symbols) caused lint to fail, blocking the commit.
- **Fix:** Implemented production code (Task 2) before committing, then committed all files together. The TDD RED→GREEN sequence was preserved in implementation order within the session.
- **Files modified:** None beyond plan — same files, same order.
- **Commit:** 7e18d11

## Verification Results

- `go test ./internal/deployer/... -race` — 632 tests pass across 3 packages
- `go test ./internal/validation/... -race` — passes (SourcePod.Namespace addition is backward-compatible)
- `task test:unit` — exits 0, total coverage 39.4%
- `internal/deployer/probe.go` uses `kubernetes.Interface` in all signatures
- `ProbePodImageVersion` has correct Renovate annotation (`datasource=docker depName=curlimages/curl`)
- `SourcePod.Namespace` has `yaml:"namespace,omitempty" json:"namespace,omitempty"` tags
- `TestDeleteProbePod_CancelledContext` passes — PROBE-03 independent-context contract verified

## Self-Check: PASSED

- FOUND: internal/deployer/probe.go
- FOUND: internal/deployer/probe_test.go
- FOUND: internal/deployer/const.go
- FOUND: internal/validation/types.go
- FOUND: commit 7e18d11
