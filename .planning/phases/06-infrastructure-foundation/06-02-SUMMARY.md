---
phase: 06-infrastructure-foundation
plan: 02
subsystem: internal/deployer
tags: [nginx-ingress, gateway-api, cloud-provider-kind, component-installer, tdd]
requirements-completed: [INFRA-01, INFRA-02, INFRA-03]
dependency_graph:
  requires: [06-01]
  provides: [installNginxIngress, installGatewayAPI, ensureCloudProviderKind]
  affects: [06-04]
tech_stack:
  added: []
  patterns:
    - ComponentResult pattern (name + status + message) for all installers
    - Two-pass REST mapper refresh for Gateway API CRD install
    - Discovery().ServerResourcesForGroupVersion() for CRD presence check
    - pgrep-based process detection for cloud-provider-kind
    - Detached process start via SysProcAttr{Setsid: true} + cmd.Start() only
    - nolint:unused for Wave 2 functions (used by plan 04 setup.go)
key_files:
  created:
    - internal/deployer/cloud_provider_kind.go
    - internal/deployer/cloud_provider_kind_test.go
  modified:
    - internal/deployer/infrastructure.go
    - internal/deployer/infrastructure_test.go
decisions:
  - "Discovery().ServerResourcesForGroupVersion() used for Gateway API CRD check — avoids apiextensions-apiserver import"
  - "cloudProviderKindBinaryURLForPlatform(goos, goarch) extracted as testable variant — cloudProviderKindBinaryURL() delegates to it"
  - "downloadCloudProviderKind uses net/http directly — kube.FetchManifest URL allowlist rejects github binary download URLs"
  - "bytes.NewReader used for gzip parsing to correctly handle binary tar.gz data"
  - "installNginxIngress type-asserts clientset to *kubernetes.Clientset for WaitForDeploymentsReady compatibility"
metrics:
  duration: 12min
  completed: "2026-03-11"
  tasks: 2
  files: 4
---

# Phase 6 Plan 02: nginx-ingress, Gateway API, and cloud-provider-kind installers Summary

Implemented Wave 2 infrastructure component installers: nginx-ingress controller, Gateway API CRDs with GatewayClass, and cloud-provider-kind binary lifecycle manager — all returning ComponentResult for uniform wiring in plan 04 setup.go.

## Tasks Completed

| Task | Description | Commit | Files |
|------|-------------|--------|-------|
| 1 | nginx-ingress and Gateway API installer functions | 638a9ff (via plan 03 merge) | infrastructure.go, infrastructure_test.go |
| 2 | cloud-provider-kind binary manager | 110453d | cloud_provider_kind.go, cloud_provider_kind_test.go |

## Functions Implemented

### infrastructure.go additions

**URL functions:**
- `nginxIngressKindManifestURL()` — Kind-specific deploy manifest URL using `NginxIngressVersion`
- `gatewayAPICRDsURL()` — Gateway API standard-install manifest URL using `GatewayAPICRDsVersion`

**Readiness checks:**
- `isNginxIngressReadyWithClient(ctx, clientset)` — checks ingress-nginx namespace + deployment ReadyReplicas
- `isGatewayAPICRDsInstalled(ctx, clientset)` — uses `Discovery().ServerResourcesForGroupVersion("gateway.networking.k8s.io/v1")`

**Installers (nolint:unused — used by plan 04):**
- `installNginxIngress(ctx, clientset, dynamicClient, mapper)` — creates namespace, fetches manifest, applies, waits for deployment
- `installGatewayAPI(ctx, clientset, dynamicClient)` — two-pass apply: CRDs then rebuild mapper then GatewayClass

**Constants:**
- `gatewayClassManifest` var — GatewayClass for cloud-provider-kind controller
- `nginxIngressNamespace` const — "ingress-nginx"

### cloud_provider_kind.go (new file)

- `cloudProviderKindBinaryURLForPlatform(goos, goarch)` — testable URL generator
- `cloudProviderKindBinaryURL()` — runtime platform delegate
- `isCloudProviderKindRunning()` — pgrep -f cloud-provider-kind
- `downloadCloudProviderKind(url, destPath)` — HTTP download, gzip+tar extraction, chmod 0755
- `startCloudProviderKindDetached(binPath)` — SysProcAttr{Setsid:true}, cmd.Start() only
- `ensureCloudProviderKind(ctx)` — idempotent: check running → download if absent → start

## Tests Added

**infrastructure_test.go:**
- `TestNginxIngressURL` (3 subtests): version, host, HTTPS
- `TestGatewayAPIURL` (3 subtests): version, host, HTTPS
- `TestIsNginxIngressReady` (4 subtests): namespace missing, deployment not found, not ready, ready
- `TestIsGatewayAPICRDsInstalled` (1 test): fake clientset returns false

**cloud_provider_kind_test.go:**
- `TestCloudProviderKindBinaryURL_LinuxAMD64`
- `TestCloudProviderKindBinaryURL_DarwinARM64`
- `TestCloudProviderKindBinaryURL_VersionStripping`: "v" in path but not in filename

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed cert-manager type mismatch blocking test compilation**
- **Found during:** Task 1 setup (GREEN phase)
- **Issue:** Plan 03 had added `installCertManager` using `*kubernetes.Clientset` in function signature but the lint tool was flagging it, and the test file referenced `waitForCertManagerWebhookEndpoints` which was undefined
- **Fix:** The plan 03 agent had already committed the fix in `638a9ff` — this was already resolved before plan 02 implementation began
- **Files modified:** internal/deployer/infrastructure.go

**2. [Rule 1 - Bug] Simplified gzip parsing in downloadCloudProviderKind**
- **Found during:** Task 2 implementation
- **Issue:** Initial implementation used `strings.NewReader` for binary gzip data (would corrupt binary), plus had unused `bytesReadCloser` type and `newBytesReader` function causing lint failures
- **Fix:** Replaced with `bytes.NewReader(data)` for correct binary handling; removed unused helper types
- **Files modified:** internal/deployer/cloud_provider_kind.go

### Context: Parallel Plan 03 Execution

Plan 03 (cert-manager) ran in parallel with plan 02 on the same working directory. The pre-commit hook lint pass during plan 03's commit captured and committed some of the plan 02 nginx/gateway functions (the functions were in the working tree at commit time). As a result:
- Plan 02's nginx/gateway functions appear in commit `638a9ff` (labeled as plan 03)
- Plan 02 committed only the cloud_provider_kind.go additions in `110453d`
- All code is correct and all tests pass — the commit attribution is the only deviation

## Self-Check

### Files Exist
- [x] internal/deployer/cloud_provider_kind.go — created
- [x] internal/deployer/cloud_provider_kind_test.go — created
- [x] internal/deployer/infrastructure.go — modified (nginx/gateway functions)
- [x] internal/deployer/infrastructure_test.go — modified (new tests)

### Commits Exist
- [x] 638a9ff — nginx-ingress and gateway-api functions (via plan 03 commit)
- [x] 110453d — cloud-provider-kind binary manager

### Tests Pass
- [x] 877 tests pass across 16 packages (`go test ./... -count=1 -short`)
- [x] task test:unit green

## Self-Check: PASSED
