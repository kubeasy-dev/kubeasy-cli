---
phase: 09-tls-validation
plan: "02"
subsystem: internal/validation
tags: [tls, connectivity, executor, tdd]
dependency_graph:
  requires: ["09-01"]
  provides: ["TLS-01", "TLS-02", "TLS-03"]
  affects: ["internal/validation/executor.go", "internal/validation/executor_test.go"]
tech_stack:
  added: ["crypto/tls", "crypto/x509", "net", "net/url"]
  patterns: ["tls.Dialer.DialContext probe", "cert.VerifyHostname", "short-circuit on TLS failure"]
key_files:
  modified:
    - internal/validation/executor.go
    - internal/validation/executor_test.go
decisions:
  - "probeTLSCert uses InsecureSkipVerify:true always to fetch raw cert metadata even for expired/self-signed certs; manual validation applied by caller"
  - "hostnameForSAN helper applies HostHeader priority for SAN matching (virtual-host routing pattern)"
  - "tlsCfg.InsecureSkipVerify = true (field assignment) does not trigger gosec G402 — nolint only needed on struct literal form"
  - "httptest cert has *.example.com DNS SAN — Test E uses myapp.other-domain.io to trigger genuine mismatch"
  - "TLS failure short-circuits before HTTP request — no 'got status 0' message leaks"
metrics:
  duration: "26min"
  completed: "2026-03-11"
  tasks: 2
  files: 2
---

# Phase 9 Plan 02: TLS Certificate Validation in checkExternalConnectivity Summary

TLS expiry (TLS-01), SAN hostname (TLS-02), and insecureSkipVerify (TLS-03) validation added to `checkExternalConnectivity` using a `tls.Dialer` probe + `cert.VerifyHostname` — no new dependencies, pure stdlib.

## What Was Built

Extended `checkExternalConnectivity` in `internal/validation/executor.go` to perform explicit TLS certificate checks when `ConnectivityCheck.TLS` is set:

1. **TLS-03 (insecureSkipVerify)**: Sets `tlsCfg.InsecureSkipVerify = true` on the HTTP transport so self-signed certs in Kind succeed.
2. **TLS-01 (validateExpiry)**: Probes the TLS cert via `tls.Dialer.DialContext` (with `InsecureSkipVerify:true` to get raw cert even when expired), then checks `cert.NotAfter`. Returns friendly `"Certificate expired on 2025-01-01 (47 days ago)"` message.
3. **TLS-02 (validateSANs)**: After the probe, calls `cert.VerifyHostname(hostname)` where `hostname` uses `HostHeader` if set (virtual-host routing), else URL hostname. Returns friendly `"Hostname \"myapp.other-domain.io\" not in SANs: [...]"` message.
4. **Short-circuit**: TLS failures return before the HTTP request — no `"got status 0"` leaks.
5. **insecureSkipVerify priority**: When `InsecureSkipVerify: true`, the cert probe step is skipped entirely.

Two helper functions extracted:
- `probeTLSCert(ctx, rawURL)` — dials TLS, returns `*x509.Certificate`
- `hostnameForSAN(target)` — resolves hostname for SAN check

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Write RED tests for TLS executor logic | 0b318a4 | executor_test.go |
| 2 | Implement TLS logic in checkExternalConnectivity (GREEN) | 78e5940 | executor.go, executor_test.go |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] httptest cert has *.example.com DNS SAN — Test E hostname adjusted**
- **Found during:** Task 2 (GREEN run)
- **Issue:** Plan specified `HostHeader: "myapp.example.com"` for SAN mismatch test, but `httptest.NewTLSServer` cert includes `*.example.com` as a DNS SAN — `VerifyHostname("myapp.example.com")` passes, causing the test to not fail as expected.
- **Fix:** Changed Test E `HostHeader` to `"myapp.other-domain.io"` which is genuinely outside the httptest cert's SANs.
- **Files modified:** internal/validation/executor_test.go
- **Commit:** 78e5940

**2. [Rule 1 - Bug] Unnecessary nolint directives removed**
- **Found during:** Task 2 (lint run)
- **Issue:** `//nolint:gosec` on `&tls.Config{}` empty literal and on field assignment `tlsCfg.InsecureSkipVerify = true` triggered "unused directive" lint errors. Gosec G402 only applies to struct literal form `InsecureSkipVerify: true`.
- **Fix:** Removed nolint from those two locations; kept it only on the probe's struct literal in `probeTLSCert`.
- **Files modified:** internal/validation/executor.go

## Pre-existing Out-of-Scope Issue

`TestConnectivityValidation_NoSourcePodSpecified_Failure` integration test fails with `"probe pod failed to become ready: context deadline exceeded"` (expects `"No source pod specified"`). This failure exists on the commit prior to this plan and is not caused by these changes.

Logged to: `.planning/phases/09-tls-validation/deferred-items.md` (pre-existing)

## Self-Check: PASSED

- internal/validation/executor.go: EXISTS
- internal/validation/executor_test.go: EXISTS (TestCheckExternalConnectivityTLS present)
- Commit 0b318a4: EXISTS (RED tests)
- Commit 78e5940: EXISTS (GREEN implementation)
- `task test:unit`: PASS (all new TLS tests GREEN, zero regressions)
- `task lint`: PASS (0 issues)
