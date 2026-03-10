# kubeasy-cli

## What This Is

`kubeasy-cli` is a Go CLI (Cobra) tool that enables developers to learn Kubernetes through practical challenges. It manages a local Kind cluster, deploys challenges via OCI artifacts from ghcr.io, and validates solutions directly against the cluster using a robust 5-type validation system (condition, status, log, event, connectivity).

The v1.0 milestone eliminated accumulated technical debt — panics, silent errors, missing tests, and security gaps — to make the validation system safe to extend with new types.

## Core Value

The validation system must be robust, extensible, and test-covered — so that adding a new validation type is simple and safe without risk of breaking existing validations.

## Requirements

### Validated

<!-- Shipped and confirmed in production -->

- ✓ Cluster Kind created and configured via `kubeasy setup` — existing
- ✓ Challenge deployment via OCI artifacts (ghcr.io) — existing
- ✓ 5-type validation system: condition, status, log, event, connectivity — existing
- ✓ Result submission to backend API — existing
- ✓ JWT authentication via system keyring — existing
- ✓ Lifecycle commands: start, submit, reset, clean — existing
- ✓ Developer tools (`dev_*` commands) with local validation — existing
- ✓ Safe comma-ok type assertions in executor — no panics on malformed spec — v1.0
- ✓ Slug validation guard before any API or cluster call in all 4 commands — v1.0
- ✓ KUBEASY_LOCAL_CHALLENGES_DIR env var replaces hardcoded developer path — v1.0
- ✓ Unit tests on start, submit, reset, clean commands (11 tests) — v1.0
- ✓ ApplyManifest fail-fast on critical errors — v1.0
- ✓ Context propagation (Ctrl-C) across all 17 api.* functions — v1.0
- ✓ KUBEASY_API_URL env var for local dev without GoReleaser — v1.0
- ✓ Canonical API function names — 6 aliases removed — v1.0
- ✓ Shared applyManifestDirs helper in deployer package — v1.0
- ✓ wait.PollUntilContextTimeout in readiness polling — v1.0
- ✓ Shell injection eliminated in executeConnectivity (buildCurlCommand) — v1.0
- ✓ FetchManifest domain allowlist (github.com / raw.githubusercontent.com only) — v1.0

### Active

- [ ] New validation type: `rbac` — test ServiceAccount permissions (VTYPE-01)
- [ ] Support CronJobs, ConfigMaps, Ingress, PVC in `getGVRForKind` (VTYPE-02)
- [ ] Metrics validation (restart count, resource usage) (VTYPE-03)
- [ ] Parallel readiness checking for multi-component challenges (PERF-01)
- [ ] REST mapper cache between deployer calls (PERF-02)
- [ ] Log streaming with bufio.Scanner for high-volume pods (OBS-01)
- [ ] Fix wget fallback in checkConnectivity (sh -c → direct args) — deferred from v1.0

### Out of Scope

- Full architectural refactor — layered structure is correct, implementations were the problem
- apigen migration — generated API client remains as-is
- Backend or challenge.yaml format changes — out of CLI scope

## Context

**Current state (post-v1.0):**
- Go 1.25.4, ~24,255 LOC across all .go files
- 826 unit tests passing, total coverage ~45.8%
- All golangci-lint checks green
- Architecture: cmd/ → internal/{api,deployer,kube,validation,constants,logger}/

**Technical debt remaining:**
- wget fallback in `checkConnectivity` (executor.go:503) still uses `sh -c` — explicitly deferred with TODO(sec)
- No Nyquist VALIDATION.md compliance yet for any phase (drafts exist but not completed)

## Constraints

- **Tech stack**: Go only — no new languages or frameworks
- **Compatibility**: All commands must continue working after each change
- **Tests**: testify (already present); setup-envtest for integration tests
- **Linting**: golangci-lint must pass after every change

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Brownfield — no architectural refactor | Layered structure is correct; only implementations were problematic | ✓ Good — clean phases without breakage |
| Tests first on critical commands | start and submit are product core — test before refactoring reduces regression risk | ✓ Good — Phase 2 tests caught nothing new but provide safety net |
| Comma-ok on all Spec assertions | Returns Result{Passed:false} with descriptive message instead of panicking | ✓ Good — 6 regression tests confirm no panics |
| Function-var injection for testability | Avoids real API/cluster calls in unit tests without introducing interfaces | ✓ Good — clean pattern, used across 3 command files |
| ui.SetCIMode(true) in TestMain | Suppresses pterm spinner goroutine data races under -race detector | ✓ Good — required for -race clean tests |
| Alias deletion (no grace period) | All callers in same repo — grace period unnecessary | ✓ Good — zero dead code |
| applyManifestDirs unexported | Only used within deployer package — no public API needed | ✓ Good — minimal surface area |
| wait.PollUntilContextTimeout | Idiomatic k8s-client-go pattern with native context cancellation | ✓ Good — replaces fragile time.After loops |
| KUBEASY_API_URL via init() | env var priority beats GoReleaser ldflags for staging without special builds | ✓ Good — simple, no flags added |
| buildCurlCommand returns arg slice | No shell invoked; escapeShellArg deleted; SEC-01 closes injection surface | ✓ Good — 5 tests lock the no-shell contract |
| fetchManifestAllowedPrefixes | Prefix check before http.Get; #nosec replaced with truthful nolint | ✓ Good — infrastructure URLs already match allowlist |
| wget fallback deferred (TODO(sec)) | Out of SEC-01 scope; documented for future | — Pending — carry to v1.1 |

---
*Last updated: 2026-03-11 after v1.0 milestone*
