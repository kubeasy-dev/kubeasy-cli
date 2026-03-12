# Milestones

## v2.7.0 Connectivity Extension (Shipped: 2026-03-11)

**Phases completed:** 4 phases, 10 plans
**Timeline:** 2026-03-11 (1 day sprint)
**Requirements:** 20/20 satisfied
**Stats:** ~79 commits · 114 files · +24,658 / -2,240 lines · 341 unit tests

**Key accomplishments:**
- `SetupAllComponents` orchestrates 6 infrastructure components (nginx-ingress, Gateway API CRDs + GatewayClass, cert-manager, cloud-provider-kind, Kyverno, local-path-provisioner) with per-component ready/not-ready/missing status output
- CLI-managed probe pod lifecycle (`curlimages/curl`) auto-deployed and cleaned up when `sourcePod` absent — enables NetworkPolicy testing without challenge authors providing a curl pod
- External HTTP mode (`mode: external`) sends requests from CLI host via `net/http` — validates Ingress and Gateway API endpoints with `hostHeader` override and sslip.io hostname support
- TLS certificate validation — expiry (`NotAfter`), SAN hostname (`VerifyHostname`), `insecureSkipVerify` for self-signed certs — pure stdlib, no new dependencies
- Kind cluster created with extraPortMappings 8080/8443; full config-diff detection triggers recreation prompt on mismatch
- Blocked-connection assertion (`expectedStatus: 0`) + `sourceNamespace` field for cross-namespace NetworkPolicy test scenarios

**Archive:** .planning/milestones/v2.7.0-ROADMAP.md

---

## v1.0 Réduction de la dette technique (Shipped: 2026-03-11)

**Phases completed:** 5 phases, 14 plans
**Timeline:** 2026-03-09 → 2026-03-11 (2 days)
**Requirements:** 16/16 satisfied

**Key accomplishments:**
- Eliminated all panic paths in the validation executor — comma-ok on 5 type assertions with 6 regression tests
- 11 new unit tests cover all 4 core commands (start, submit, reset, clean) via function-var injection
- ApplyManifest now surfaces critical errors to users — fail-fast replaces silent nil returns
- Ctrl-C cancels in-flight API requests — context propagated to all 17 api.* functions
- Removed 6 backward-compat API aliases and de-duplicated walk-and-apply logic across deployers
- Eliminated shell injection in connectivity validation; FetchManifest restricted to trusted GitHub domains

**Archive:** .planning/milestones/v1.0-ROADMAP.md

---

