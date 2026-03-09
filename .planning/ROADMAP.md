# Roadmap: kubeasy-cli — Réduction de la dette technique

## Overview

Five phases that progressively harden the validation system: first eliminate crash-causing panics and missing slug guards, then lock in test coverage on the now-safe commands, then fix silent error swallowing and context propagation, then remove dead code and duplication, and finally close two security gaps. Each phase leaves the CLI fully functional and `golangci-lint` green before the next begins.

## Phases

- [x] **Phase 1: Safety Hardening** - Eliminate panics in the executor and enforce slug validation on all production commands (completed 2026-03-09)
- [ ] **Phase 2: Command Test Coverage** - Unit tests for the four core user-facing commands and their error paths
- [x] **Phase 3: Error Handling** - Surface manifest errors, propagate Ctrl-C cancellation, and fix the localhost URL default (completed 2026-03-09)
- [ ] **Phase 4: Code Quality** - Remove alias proliferation, deduplicate walk-and-apply logic, replace fixed polling with backoff
- [ ] **Phase 5: Security Hardening** - Replace shell injection surface in connectivity validation and restrict FetchManifest URLs

## Phase Details

### Phase 1: Safety Hardening
**Goal**: The executor never panics on a malformed spec, and no production command accepts an invalid slug
**Depends on**: Nothing (first phase)
**Requirements**: SAFE-01, SAFE-02, SAFE-03, TST-04, TST-05
**Success Criteria** (what must be TRUE):
  1. Passing a malformed validation spec through the executor returns a `Result` with `Passed: false` and a descriptive message — the CLI does not crash
  2. Running `kubeasy challenge start`, `submit`, `reset`, or `clean` with a slug containing uppercase letters or spaces returns an immediate error before any API or cluster call is made
  3. A production build does not load challenge YAML from `~/Workspace/kubeasy/challenges/` — it fetches from GitHub or uses an explicit flag/env var
  4. Unit tests verify that `getGVRForKind` returns a clear error for unsupported kinds without panicking
  5. Unit tests verify that `FindLocalChallengeFile` does not resolve the developer hardcoded path in production builds
**Plans**: 3 plans

Plans:
- [ ] 01-01-PLAN.md — Write failing tests (RED) for all 5 safety behaviors
- [ ] 01-02-PLAN.md — Fix Execute() bare type assertions with comma-ok (SAFE-01, TST-04)
- [ ] 01-03-PLAN.md — Remove hardcoded loader path + add slug validation to 3 commands (SAFE-02, SAFE-03, TST-05)

### Phase 2: Command Test Coverage
**Goal**: The four core production commands have unit tests that catch regressions in their primary flows and error paths
**Depends on**: Phase 1
**Requirements**: TST-01, TST-02, TST-03
**Success Criteria** (what must be TRUE):
  1. Running `task test:unit` exercises the `RunE` of `cmd/start.go` including slug validation, progress state machine, and API call sequence
  2. Running `task test:unit` exercises the `RunE` of `cmd/submit.go` including validation loading, execution, and result submission
  3. Running `task test:unit` exercises the `RunE` of `cmd/reset.go` and `cmd/clean.go` including their error paths
  4. A simulated API failure in any of the four commands causes the test to assert a non-nil error return from `RunE` — not a panic
**Plans**: 3 plans

Plans:
- [ ] 02-01-PLAN.md — Add function vars to start.go + write start_test.go (TST-01)
- [ ] 02-02-PLAN.md — Add function vars to submit.go + write submit_test.go (TST-02)
- [ ] 02-03-PLAN.md — Fix reset.go + write reset_test.go and clean_test.go (TST-03)

### Phase 3: Error Handling
**Goal**: Errors from manifest application are surfaced to the user, Ctrl-C cancels in-flight API requests immediately, and local builds can point at a real backend without GoReleaser
**Depends on**: Phase 2
**Requirements**: ERR-01, ERR-02, ERR-03
**Success Criteria** (what must be TRUE):
  1. When a manifest fails to apply during `kubeasy challenge start`, the command exits with a non-zero code and a user-visible error message — not silent success
  2. Pressing Ctrl-C during `kubeasy challenge start` or `submit` cancels the in-flight HTTP request within one second; the CLI exits cleanly rather than hanging for 30 seconds
  3. Setting `KUBEASY_API_URL=https://staging.kubeasy.com go run main.go challenge get <slug>` reaches the staging backend without requiring a GoReleaser build
**Plans**: 3 plans

Plans:
- [ ] 03-01-PLAN.md — Fix ApplyManifest critical error handling + manifest_test.go (ERR-01)
- [ ] 03-02-PLAN.md — Add KUBEASY_API_URL init() + const_test.go (ERR-03)
- [ ] 03-03-PLAN.md — Thread ctx through all api.* functions and cmd/ callers (ERR-02)

### Phase 4: Code Quality
**Goal**: The API package exposes one name per operation, manifest walking is not duplicated between deployers, and readiness polling uses backoff
**Depends on**: Phase 3
**Requirements**: QUAL-01, QUAL-02, QUAL-03
**Success Criteria** (what must be TRUE):
  1. `internal/api/client.go` has no alias functions; every caller in `cmd/` uses the single canonical function name for each API operation
  2. The walk-and-apply directory traversal logic exists in exactly one place in `internal/deployer/`; `challenge.go` and `local.go` both call the shared helper
  3. `WaitForDeploymentsReady` and `WaitForStatefulSetsReady` use `wait.PollUntilContextTimeout` with backoff — no `time.Sleep` in a fixed loop
**Plans**: TBD

### Phase 5: Security Hardening
**Goal**: Connectivity validation uses no shell and FetchManifest cannot be called with arbitrary URLs
**Depends on**: Phase 4
**Requirements**: SEC-01, SEC-02
**Success Criteria** (what must be TRUE):
  1. `executeConnectivity` passes the target URL as a positional argument to `exec.Command("curl", ...)` — no `sh -c` string is constructed or executed
  2. `FetchManifest` either becomes unexported or rejects URLs not matching a trusted allowlist, preventing callers from fetching arbitrary remote content
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Safety Hardening | 3/3 | Complete   | 2026-03-09 |
| 2. Command Test Coverage | 1/3 | In Progress|  |
| 3. Error Handling | 3/3 | Complete   | 2026-03-09 |
| 4. Code Quality | 0/? | Not started | - |
| 5. Security Hardening | 0/? | Not started | - |
