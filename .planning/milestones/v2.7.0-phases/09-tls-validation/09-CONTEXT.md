# Phase 9: TLS Validation - Context

**Gathered:** 2026-03-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Add TLS certificate validation to external connectivity checks (`mode: external`). Challenge authors can validate that a server cert is not expired (TLS-01), that the hostname matches the cert SANs (TLS-02), and can opt out of CA trust verification for self-signed Kind/cert-manager certs (TLS-03). Internal pod-exec mode is out of scope.

</domain>

<decisions>
## Implementation Decisions

### TLS spec shape
- New `tls:` sub-block on `ConnectivityCheck` (per-target, not spec-level) — consistent with `hostHeader` placement
- Fields: `insecureSkipVerify: bool`, `validateExpiry: bool`, `validateSANs: bool`
- The `tls:` block is optional — omitting it means no explicit TLS checks are requested

### TLS validation trigger
- Explicit opt-in via the `tls:` block — TLS-01 and TLS-02 checks only run when `validateExpiry: true` or `validateSANs: true`
- Without a `tls:` block, an `https://` URL still works: Go's standard TLS validates everything automatically (CA trust, expiry, hostname), but error messages are raw Go errors (not friendly)
- With `tls: { validateExpiry: true }`: expiry is checked and a friendly message is produced on failure
- With `tls: { validateSANs: true }`: SAN hostname matching is checked with a friendly message on failure

### insecureSkipVerify semantics
- `insecureSkipVerify: true` → skip ALL cert checks: CA trust, expiry, AND hostname SANs
- `insecureSkipVerify: true` takes priority and wins over `validateExpiry`/`validateSANs` — no manual cert inspection runs
- Primary use case: cert-manager self-signed certs in Kind (trusted by challenge infra but not by OS trust store)

### TLS error messages
- Friendly + cert metadata when `validateExpiry` or `validateSANs` is true:
  - Expiry failure: `"Certificate expired on 2025-01-01 (2 months ago)"`
  - SAN mismatch: `"Hostname 'myapp.example.com' not in SANs: [myapp.sslip.io, *.sslip.io]"`
- TLS failure short-circuits the HTTP status code check — do not report `"got status 0, expected 200"` when TLS is the actual failure

### Claude's Discretion
- Whether to use a separate `tls.Dial` probe to fetch cert metadata for friendly messages vs parsing Go TLS error strings
- Whether `TLSConfig` is a struct pointer (`*TLSConfig`) or inline fields on `ConnectivityCheck`
- Test strategy for TLS: httptest.TLSServer vs custom crypto/tls cert generation

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `ConnectivityCheck` in `types.go` — add `TLS *TLSConfig` field (or inline fields) here
- `checkExternalConnectivity` in `executor.go` — integration point; currently creates `http.Client{}` with no TLS config; Phase 9 adds `tls.Config` to the transport
- `checkExternalConnectivityAll` in `executor.go` — iterates targets; no change needed at this level
- `http.Transport` with `tls.Config{InsecureSkipVerify: true}` — standard Go pattern for bypassing CA trust

### Established Patterns
- `insecureSkipVerify` was explicitly deferred from Phase 8 as a future `ConnectivityCheck` field (per Phase 8 CONTEXT.md deferred section)
- Per-target fields (e.g., `HostHeader`, `TimeoutSeconds`) live on `ConnectivityCheck` — `tls:` block follows the same pattern
- `req.Host` override for virtual-host routing — already established in Phase 8 for `hostHeader`
- `resp.TLS.PeerCertificates[0]` — available post-handshake for cert inspection (`NotAfter`, `DNSNames`)

### Integration Points
- `internal/validation/types.go` → add `TLS` field to `ConnectivityCheck`; new `TLSConfig` struct
- `internal/validation/executor.go` → `checkExternalConnectivity()` builds `tls.Config` from `target.TLS`, inspects `resp.TLS` for cert metadata when producing friendly errors
- `internal/validation/loader.go` → may need to validate `tls` block fields at parse time (e.g., `insecureSkipVerify: true` + `validateExpiry: true` is valid but insecureSkipVerify wins)

</code_context>

<specifics>
## Specific Ideas

- The `tls:` block is the challenge author's explicit contract: they declare what TLS properties matter for the challenge
- `insecureSkipVerify: true` is the Kind/cert-manager pattern — it bypasses CA chain validation while still allowing the HTTP request to complete
- TLS failure must short-circuit the HTTP status check to avoid confusing error messages like "got status 0, expected 200"

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 09-tls-validation*
*Context gathered: 2026-03-11*
