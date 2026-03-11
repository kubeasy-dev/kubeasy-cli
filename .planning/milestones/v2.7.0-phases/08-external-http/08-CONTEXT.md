# Phase 8: External HTTP - Context

**Gathered:** 2026-03-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Add `mode: external` to the connectivity validation type so the CLI sends HTTP requests directly from the host process (via `net/http`), without pod exec. Supports `hostHeader` for virtual-host routing when the URL uses a direct IP. sslip.io hostnames (e.g., `myapp.127-0-0-1.sslip.io:8080`) resolve naturally through DNS — no special resolution logic. Phase 8 is HTTP-only; TLS validation is Phase 9.

</domain>

<decisions>
## Implementation Decisions

### Mode field placement
- `mode` lives at `ConnectivitySpec` level — same level as `sourcePod`
- Default: `internal` when field is absent (full backwards compatibility with existing challenges)
- `mode: external` → CLI uses `net/http` from the host process, no pod exec, `sourcePod` is not consulted
- `mode: external` + `sourcePod` defined → validation error (spec is incoherent, fail early)

### hostHeader
- Per-target field on `ConnectivityCheck` (optional, absent = Host header derived from URL)
- Overrides the HTTP `Host` header sent with the request
- Primary use case: URL contains a direct IP (e.g., `http://127.0.0.1:8080/`) + Ingress routes by hostname → `hostHeader: myapp.example.com`
- With sslip.io URLs, Host header is the sslip.io hostname automatically — `hostHeader` not needed in that case

### TLS boundary
- Phase 8 is HTTP-only (`http://` URLs, port 8080)
- No HTTPS handling, no `insecureSkipVerify` — that is Phase 9 scope
- Challenge authors using Phase 8 external mode write `http://` URLs

### Claude's Discretion
- Exact `net/http` client setup (timeout propagation from `TimeoutSeconds`, redirect policy)
- Error message wording for failed external requests (connection refused, DNS failure, wrong status)
- Whether to reuse a single `http.Client` or create per-request
- Loader/parser validation logic for the `mode` field

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `ConnectivitySpec` in `types.go` — add `Mode string` field here (same level as `SourcePod`)
- `ConnectivityCheck` in `types.go` — add `HostHeader string` field here (optional, per-target)
- `executeConnectivity` in `executor.go` — branch on `spec.Mode == "external"` before the sourcePod switch; call a new `checkExternalConnectivity` method
- `checkConnectivity` in `executor.go` — existing internal check method; unchanged

### Established Patterns
- `mode` as string discriminant (not a new `ValidationType`) — matches STATE.md decision, preserves backend compat
- `buildCurlCommand` no-shell contract locked for internal; external mode uses `net/http` directly
- `context.WithTimeout` wrapping for timeouts — established in Phase 6 and 7
- Test environment guard: `e.restConfig.Host == ""` pattern already used in `checkConnectivity` — replicate for external if needed

### Integration Points
- `internal/validation/types.go` → add `Mode` to `ConnectivitySpec`, `HostHeader` to `ConnectivityCheck`
- `internal/validation/executor.go` → `executeConnectivity()` branches on mode; new `checkExternalConnectivity()` method uses `net/http`
- `internal/validation/loader.go` → may need to validate `mode` value on parse

</code_context>

<specifics>
## Specific Ideas

- sslip.io hostnames like `myapp.127-0-0-1.sslip.io:8080` encode 127.0.0.1 — DNS resolves them without local config. `net/http` handles this transparently. No special resolution logic needed.
- The `mode: external` + `sourcePod` error should be a loader/parse-time error, not a runtime error — fail fast before any cluster calls.

</specifics>

<deferred>
## Deferred Ideas

- `insecureSkipVerify: true` for HTTPS Kind self-signed certs — Phase 9 (TLS-03)
- HTTPS external checks — Phase 9 scope

</deferred>

---

*Phase: 08-external-http*
*Context gathered: 2026-03-11*
