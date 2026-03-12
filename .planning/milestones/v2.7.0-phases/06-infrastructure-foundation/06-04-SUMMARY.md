---
phase: 06-infrastructure-foundation
plan: 04
subsystem: infra
tags: [kind, kubernetes, kyverno, nginx-ingress, gateway-api, cert-manager, cloud-provider-kind, cobra]

# Dependency graph
requires:
  - phase: 06-01
    provides: writeKindConfigToPath, hasExtraPortMappingsAt, ComponentResult, ComponentStatus types
  - phase: 06-02
    provides: installNginxIngress, installGatewayAPI, ensureCloudProviderKind, WriteKindConfig, HasExtraPortMappings
  - phase: 06-03
    provides: installCertManager, isCertManagerReadyWithClient, waitForCertManagerWebhookEndpoints
provides:
  - SetupAllComponents — orchestrates all 6 infrastructure component installers into a single call
  - installKyverno — per-component installer with idempotency check returning ComponentResult
  - installLocalPathProvisioner — per-component installer with idempotency check returning ComponentResult
  - isKyvernoReadyWithClient — extracted Kyverno readiness check (testable)
  - isLocalPathProvisionerReadyWithClient — extracted local-path-provisioner readiness check (testable)
  - WriteKindConfig, HasExtraPortMappings — exported wrappers (moved from plan 02 stubs)
  - cmd/setup.go fully integrated with per-component status output, Kind config detection, cluster recreation prompt
  - cmd/demo_start.go updated to use SetupAllComponents (removes deprecated SetupInfrastructure calls)
affects:
  - Phase 07 (probe pod lifecycle uses deployer package)
  - Phase 08 (connectivity validation depends on nginx-ingress being ready)
  - Any future plan touching setup.go or infrastructure deployment

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Per-component installer pattern: each component returns ComponentResult, SetupAllComponents collects all 6
    - Idempotency-first install: every installer checks readiness before attempting install
    - Exported wrapper thin-wraps internal path-specific variants (WriteKindConfig → writeKindConfigToPath)
    - printComponentResult dispatches on ComponentStatus to ui.Success/Error/Warning
    - CreateWithV1Alpha4Config for Kind cluster creation with extraPortMappings

key-files:
  created: []
  modified:
    - internal/deployer/infrastructure.go
    - internal/deployer/infrastructure_test.go
    - internal/deployer/cloud_provider_kind.go
    - cmd/setup.go
    - cmd/demo_start.go

key-decisions:
  - "SetupAllComponents accepts *kubernetes.Clientset (not Interface) to satisfy installCertManager and WaitForDeploymentsReady signatures"
  - "SetupAllComponents builds one REST mapper upfront; Gateway API rebuilds its own mapper internally after CRD install (two-pass)"
  - "printComponentResult lives in cmd/setup.go (not deployer/) — it is a UI concern, not a deployment concern"
  - "SetupInfrastructure kept (not deleted) for backward compat — only callers updated (demo_start.go, setup.go)"
  - "Human verification run live: all 6 components showed 'ready' status lines after kubeasy setup"

patterns-established:
  - "Per-component installer pattern: func install*(ctx, clientset, dynamicClient, mapper) ComponentResult — check readiness first, install if needed, return ComponentResult"
  - "SetupAllComponents continues regardless of individual failures — all 6 results always returned"
  - "Footer always prints after SetupAllComponents even if some components are not-ready"

requirements-completed: [INFRA-05, INFRA-06, INFRA-07]

# Metrics
duration: 25min
completed: 2026-03-11
---

# Phase 6 Plan 4: Infrastructure Integration Summary

**SetupAllComponents wiring all 6 component installers (kyverno, local-path-provisioner, nginx-ingress, gateway-api, cert-manager, cloud-provider-kind) with per-component status output in setup.go and Kind cluster extraPortMappings 8080/8443 via CreateWithV1Alpha4Config**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-03-11T09:27:08Z
- **Completed:** 2026-03-11T10:41:13Z
- **Tasks:** 3 (2 auto + 1 human-verify checkpoint)
- **Files modified:** 5

## Accomplishments

- Extracted `isKyvernoReadyWithClient` and `isLocalPathProvisionerReadyWithClient` from the monolithic `IsInfrastructureReadyWithClient`, enabling per-component readiness checks
- Added `installKyverno` and `installLocalPathProvisioner` returning `ComponentResult` with idempotency (check-before-install)
- Added `SetupAllComponents` orchestrating all 6 installers — never aborts on individual failure, always returns 6 results
- Rewrote `cmd/setup.go`: cluster creation uses `CreateWithV1Alpha4Config` with extraPortMappings; existing cluster without correct config triggers `ui.Confirmation` prompt before recreation; infrastructure step replaced with per-component `printComponentResult` output
- Updated `cmd/demo_start.go` to use `createClusterWithConfig` and `SetupAllComponents`, removing all deprecated `SetupInfrastructure` calls
- Human-verified live: all 6 components showed "ready" status lines; kind-config.yaml written with 8080/8443 port mappings

## Task Commits

Each task was committed atomically:

1. **Task 1: Retrofit existing components and add SetupAllComponents** - `50cad35` (feat)
2. **Task 2: Rewrite setup.go with Kind config, port mapping detection, and per-component output** - `7a57892` (feat)
3. **Task 3: Human verify checkpoint** - approved (no code commit — verification only)
4. **Post-verify refactor: Replace port-mapping check with full config diff** - `683fbf7` (refactor)

## Files Created/Modified

- `internal/deployer/infrastructure.go` — Added `isKyvernoReadyWithClient`, `isLocalPathProvisionerReadyWithClient`, `installKyverno`, `installLocalPathProvisioner`, `SetupAllComponents`, `WriteKindConfig`, `HasExtraPortMappings`; removed `nolint:unused` from all functions now actively called
- `internal/deployer/infrastructure_test.go` — Added `TestInstallKyverno_AlreadyReady`, `TestInstallLocalPathProvisioner_AlreadyReady`, `TestIsKyvernoReady_*`, `TestIsLocalPathProvisionerReady_*`
- `internal/deployer/cloud_provider_kind.go` — Removed `nolint:unused` directives from all functions now called by `SetupAllComponents`
- `cmd/setup.go` — Full rewrite: `kindClusterConfig()`, `createClusterWithConfig()`, `printComponentResult()`, `setupCmd` with port-mapping detection and per-component output
- `cmd/demo_start.go` — Updated to `createClusterWithConfig` and `SetupAllComponents`; removed deprecated `SetupInfrastructure` calls

## Decisions Made

- `SetupAllComponents` accepts `*kubernetes.Clientset` (not `Interface`) to satisfy `installCertManager` and `kube.WaitForDeploymentsReady` signatures — both require the concrete type
- `SetupAllComponents` builds one REST mapper upfront; `installGatewayAPI` rebuilds its own mapper after CRD install (two-pass apply for Gateway API CRDs)
- `printComponentResult` lives in `cmd/setup.go` — it is a UI concern, not a deployment concern
- `SetupInfrastructure` kept (not deleted) — only callers updated to eliminate deprecation warnings from the linter

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated demo_start.go to remove deprecated SetupInfrastructure calls**
- **Found during:** Task 2 (commit attempt for setup.go)
- **Issue:** `cmd/demo_start.go` still called `deployer.SetupInfrastructure` which triggered `staticcheck SA1019` deprecation warning, failing the pre-commit hook
- **Fix:** Updated `demo_start.go` to call `createClusterWithConfig` and `SetupAllComponents`; removed the duplicate `kube.GetKubernetesClient()` call that was now redundant; removed unused `cluster` import
- **Files modified:** `cmd/demo_start.go`
- **Verification:** `go build ./cmd/...` succeeded; lint passed
- **Committed in:** `7a57892` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** The fix was necessary to satisfy the linter pre-commit hook. It improved `demo_start.go` as a side effect by migrating it to the new `SetupAllComponents` path. No scope creep.

## Issues Encountered

- `gocritic deprecatedComment` lint rule requires `Deprecated:` notices in a separate paragraph (blank comment line before), not inline — fixed by restructuring the doc comments
- `staticcheck SA1019` flagged callers of the deprecated `SetupInfrastructure` in `demo_start.go` — resolved by updating that file as part of Task 2

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Phase 6 infrastructure foundation is complete: all 6 components (kyverno, local-path-provisioner, nginx-ingress, gateway-api, cert-manager, cloud-provider-kind) have idempotent installers wired into `SetupAllComponents`
- Phase 7 (probe pod lifecycle) can proceed — `deployer` package is stable and well-tested
- Concern: Phase 8 macOS Docker IP reachability with cloud-provider-kind v0.10.0 remains MEDIUM confidence — verify locally before finalizing EXT-03 NodePort fallback

---
*Phase: 06-infrastructure-foundation*
*Completed: 2026-03-11*
