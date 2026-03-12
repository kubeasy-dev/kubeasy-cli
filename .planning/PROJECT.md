# kubeasy-cli

## What This Is

`kubeasy-cli` is a Go CLI (Cobra) tool that enables developers to learn Kubernetes through practical challenges. It manages a local Kind cluster (with nginx-ingress, Gateway API, cert-manager, and cloud-provider-kind), deploys challenges via OCI artifacts from ghcr.io, and validates solutions directly against the cluster using a robust 5-type validation system (condition, status, log, event, connectivity).

The connectivity validation type now supports three execution modes: pod exec (internal), CLI-managed probe pod (NetworkPolicy testing), and direct HTTP from the CLI host (external Ingress/Gateway API + TLS inspection).

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
- ✓ `kubeasy setup` installs nginx-ingress controller (INFRA-01) — v2.7.0
- ✓ `kubeasy setup` installs Gateway API v1 CRDs + GatewayClass (INFRA-02/03) — v2.7.0
- ✓ `kubeasy setup` installs cert-manager with webhook polling (INFRA-04) — v2.7.0
- ✓ cloud-provider-kind binary auto-downloaded and started in background (INFRA-05) — v2.7.0
- ✓ Kind cluster created with extraPortMappings 8080/8443; config-diff recreation prompt (INFRA-06) — v2.7.0
- ✓ `kubeasy setup` reports per-component status (ready/not-ready/missing) (INFRA-07) — v2.7.0
- ✓ CLI auto-deploys probe pod when sourcePod absent; cleaned up with independent context (PROBE-01/03) — v2.7.0
- ✓ Probe pod namespace configurable via `probeNamespace` field (PROBE-02) — v2.7.0
- ✓ wget sh -c fallback removed — curl only (PROBE-04) — v2.7.0
- ✓ Connectivity validation supports expectedStatus 0 (blocked connection) (CONN-01) — v2.7.0
- ✓ Source pod namespace configurable for cross-namespace NetworkPolicy tests (CONN-02) — v2.7.0
- ✓ External connectivity mode — CLI HTTP request via net/http with hostHeader (EXT-01/02) — v2.7.0
- ✓ sslip.io hostname support for Ingress/Gateway API routing without local DNS (EXT-03) — v2.7.0
- ✓ External check validates expected HTTP status code (EXT-04) — v2.7.0
- ✓ TLS: certificate expiry, SAN hostname matching, insecureSkipVerify (TLS-01/02/03) — v2.7.0

### Active

<!-- Backlog deferred — not yet assigned to a milestone -->

- New validation type: `rbac` — test ServiceAccount permissions (VTYPE-01)
- Support CronJobs, ConfigMaps, Ingress, PVC in `getGVRForKind` (VTYPE-02)
- Metrics validation (restart count, resource usage) (VTYPE-03)
- Parallel readiness checking for multi-component challenges (PERF-01)
- REST mapper cache between deployer calls (PERF-02)
- Log streaming with bufio.Scanner for high-volume pods (OBS-01)
- EXT-03 macOS Docker IP routing: document sslip.io 127.x.x.x approach in challenge authoring guide

### Out of Scope

- Full architectural refactor — layered structure is correct, implementations were the problem
- apigen migration — generated API client remains as-is
- Backend or challenge.yaml format changes — out of CLI scope
- New ValidationType (TypeExternal, TypeTLS) — mode discriminant on ConnectivitySpec is sufficient
- Auto-installation of cloud-provider-kind via sudo — detection + binary download + advisory only
- Server-Side Apply (SSA) in ApplyManifest — not required for current use cases
- NGINX Gateway Fabric (NGF) — migration is a future milestone
- Ephemeral debug containers for probe — dedicated probe pod is simpler

## Context

**Current state (post-v2.7.0):**
- Go 1.25.4, ~12,189 LOC production Go across all packages
- 341 unit tests + integration test suite; total coverage ~37.3% (unit), higher with integration
- All golangci-lint checks green; all tests pass
- Architecture: cmd/ → internal/{api,deployer,kube,validation,constants,logger}/
- Connectivity validation: 3 modes — pod exec, probe pod, external HTTP + TLS

**Known tech debt:**
- EXT-03: sslip.io hostnames encoding 127.0.0.1 may not route to Kind node IP on macOS Docker Desktop — challenge authors should use hostPort approach; CLI warning not yet implemented
- Integration test `TestConnectivityValidation_NoSourcePodSpecified_Failure` was stale (now fixed)

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
| wait.PollUntilContextTimeout | Idiomatic k8s-client-go pattern with native context cancellation | ✓ Good — replaces fragile time.After loops |
| buildCurlCommand returns arg slice | No shell invoked; escapeShellArg deleted; SEC-01 closes injection surface | ✓ Good — 5 tests lock the no-shell contract |
| fetchManifestAllowedPrefixes | Prefix check before http.Get; #nosec replaced with truthful nolint | ✓ Good — infrastructure URLs already match allowlist |
| ComponentResult pattern for installers | Uniform name+status+message return from all 6 component installers | ✓ Good — enables clean per-component status output in setup.go |
| Probe pod with curlimages/curl | Dedicated pod simpler than ephemeral debug containers; no NET_RAW needed | ✓ Good — CreateProbePod/DeleteProbePod lifecycle clean and testable |
| Probe cleanup via context.Background() + 10s | Independent of caller context — survives Ctrl-C | ✓ Good — PROBE-03 contract guaranteed |
| mode: external discriminant on ConnectivitySpec | No new ValidationType — avoids breaking backend/challenge.yaml on 3 repos | ✓ Good — backward compatible, parse-time validated |
| sslip.io for external routing | No local DNS config needed; works with CLI net/http naturally | ⚠️ Revisit — macOS Docker Desktop may not route 127.x.x.x.sslip.io to Kind node |
| TLS via tls.Dialer + cert.VerifyHostname | Pure stdlib — no new deps; expiry/SAN checks before HTTP request | ✓ Good — 8 unit tests via x509 test certs |
| Two-pass apply for Gateway API CRDs | CRDs first, then rebuild mapper, then GatewayClass — required by dynamic mapper | ✓ Good — pattern established for future multi-resource installs |

---
*Last updated: 2026-03-11 after v2.7.0 milestone*
