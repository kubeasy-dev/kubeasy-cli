# Phase 8: External HTTP - Research

**Researched:** 2026-03-11
**Domain:** Go net/http, connectivity validation, sslip.io DNS, macOS Docker routing
**Confidence:** HIGH

## Summary

Phase 8 adds `mode: external` to the connectivity validation type. Instead of shelling into a pod via SPDY exec, the CLI sends HTTP requests directly from the host process using Go's standard `net/http` library. The only two files requiring new logic are `internal/validation/types.go` (add `Mode` and `HostHeader` fields) and `internal/validation/executor.go` (branch in `executeConnectivity`, new `checkExternalConnectivity` method). `loader.go` needs a small validation addition to reject the `mode: external` + `sourcePod` combination at parse time.

sslip.io hostnames like `myapp.127-0-0-1.sslip.io:8080` resolve to 127.0.0.1 via public DNS — this is a pure DNS trick with no local config, no `/etc/hosts` edits, and no special resolution logic in the CLI. On macOS with Kind using `extraPortMappings` (INFRA-06, already shipped), port 8080 on 127.0.0.1 is forwarded to the nginx-ingress controller inside the cluster. `net/http` hits 127.0.0.1:8080, and the Ingress routes by Host header. This is the happy path for all sslip.io-based challenges — no fallback needed.

**Primary recommendation:** Implement `checkExternalConnectivity` as a thin wrapper around `net/http` with a single `http.Client` per call (no shared client state), `context.WithTimeout` propagated from `TimeoutSeconds`, no redirect following (use `http.Client{CheckRedirect: noRedirect}`), and `HostHeader` overriding `req.Host` (not the `Host` request header map entry). For `mode: external` + `sourcePod` defined, return a parse-time error from `parseSpec`.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- `mode` lives at `ConnectivitySpec` level — same level as `sourcePod`
- Default: `internal` when field is absent (full backwards compatibility with existing challenges)
- `mode: external` → CLI uses `net/http` from the host process, no pod exec, `sourcePod` is not consulted
- `mode: external` + `sourcePod` defined → validation error (spec is incoherent, fail early)
- `hostHeader` is a per-target field on `ConnectivityCheck` (optional, absent = Host header derived from URL)
- Overrides the HTTP `Host` header sent with the request
- Primary use case: URL contains a direct IP + Ingress routes by hostname → `hostHeader: myapp.example.com`
- With sslip.io URLs, Host header is the sslip.io hostname automatically — `hostHeader` not needed
- Phase 8 is HTTP-only (`http://` URLs, port 8080) — no HTTPS, no `insecureSkipVerify`
- sslip.io hostnames like `myapp.127-0-0-1.sslip.io:8080` resolve naturally via DNS — no special resolution logic

### Claude's Discretion

- Exact `net/http` client setup (timeout propagation from `TimeoutSeconds`, redirect policy)
- Error message wording for failed external requests (connection refused, DNS failure, wrong status)
- Whether to reuse a single `http.Client` or create per-request
- Loader/parser validation logic for the `mode` field

### Deferred Ideas (OUT OF SCOPE)

- `insecureSkipVerify: true` for HTTPS Kind self-signed certs — Phase 9 (TLS-03)
- HTTPS external checks — Phase 9 scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| EXT-01 | User can validate external HTTP connectivity with `mode: external` — CLI uses `net/http` (not pod exec) | `net/http` pattern with `http.NewRequestWithContext`, `http.Client{Timeout}`, verified below |
| EXT-02 | External check supports `hostHeader` for Ingress/Gateway virtual-host routing | `req.Host = spec.HostHeader` pattern — overrides Host header correctly in Go |
| EXT-03 | Challenge spec can use sslip.io URLs (`myapp.127-0-0-1.sslip.io:8080`) — CLI resolves naturally | DNS-only trick, net/http resolves transparently; Kind extraPortMappings already ship (INFRA-06) |
| EXT-04 | External check validates expected HTTP status code | Same `ExpectedStatusCode` field as internal mode, extended to external branch |
</phase_requirements>

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `net/http` (stdlib) | Go 1.26 | HTTP client for external requests | Already imported in `loader.go`; zero new deps |
| `context` (stdlib) | Go 1.26 | Timeout propagation via `context.WithTimeout` | Established pattern in Phases 6 and 7 |

### Supporting

No new external dependencies. The entire implementation lives within existing imports.

**Installation:** No new packages. Zero `go get` commands required.

---

## Architecture Patterns

### Recommended Changes

```
internal/validation/
├── types.go          # Add Mode string to ConnectivitySpec; HostHeader string to ConnectivityCheck
├── executor.go       # Branch in executeConnectivity; new checkExternalConnectivity method
└── loader.go         # Add mode validation in parseSpec (TypeConnectivity case)
```

No new files, no new packages.

### Pattern 1: Mode Field in ConnectivitySpec

**What:** A `Mode string` field discriminates between internal (pod exec) and external (host net/http) at the `ConnectivitySpec` level.

**When to use:** Always — added as optional field with empty string defaulting to `internal` behavior.

```go
// Source: CONTEXT.md locked decision + existing types.go pattern
type ConnectivitySpec struct {
    // Mode controls how connectivity checks are executed.
    // "internal" (default, empty): existing pod exec via curl (SPDY)
    // "external": CLI host sends HTTP request via net/http — no pod exec
    Mode string `yaml:"mode,omitempty" json:"mode,omitempty"`

    // SourcePod is only consulted when Mode is "internal" (or empty).
    SourcePod SourcePod `yaml:"sourcePod" json:"sourcePod"`

    Targets []ConnectivityCheck `yaml:"targets" json:"targets"`
}
```

### Pattern 2: HostHeader Field in ConnectivityCheck

**What:** A `HostHeader string` field on `ConnectivityCheck` overrides the HTTP `Host` header for per-target virtual-host routing.

```go
// Source: CONTEXT.md locked decision
type ConnectivityCheck struct {
    URL                string `yaml:"url" json:"url"`
    ExpectedStatusCode int    `yaml:"expectedStatusCode" json:"expectedStatusCode"`
    TimeoutSeconds     int    `yaml:"timeoutSeconds,omitempty" json:"timeoutSeconds,omitempty"`

    // HostHeader overrides the HTTP Host header sent with the request.
    // Use when URL is a direct IP but Ingress routes by hostname.
    // Absent: Host is derived from URL automatically by net/http.
    HostHeader string `yaml:"hostHeader,omitempty" json:"hostHeader,omitempty"`
}
```

### Pattern 3: executeConnectivity Branch

**What:** Before the `SourcePod` switch, check `spec.Mode == "external"` and delegate to the new method.

```go
// Source: CONTEXT.md code_context — established branching pattern
func (e *Executor) executeConnectivity(ctx context.Context, spec ConnectivitySpec) (bool, string, error) {
    // EXT-01: external mode short-circuits before any SourcePod resolution
    if spec.Mode == "external" {
        return e.checkExternalConnectivityAll(ctx, spec)
    }

    // existing internal/probe path unchanged below...
    sourceNamespace := e.namespace
    // ...
}
```

### Pattern 4: checkExternalConnectivity with net/http

**What:** Creates a per-request `http.Client` with context-derived timeout, sets `req.Host` for virtual-host routing, handles response code comparison.

**Key Go specifics (HIGH confidence — stdlib behavior):**

- **`req.Host`** overrides the `Host` header sent on the wire. Setting `req.Header.Set("Host", h)` does NOT work for the Host header; only `req.Host = h` does.
- **`http.Client{CheckRedirect: noRedirectFunc}`** prevents automatic redirect following. Without this, a 301 from an Ingress gets transparently followed and the test sees a 200 instead of 301. Use `func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }` to return the redirect response as-is.
- **Per-request client** is the right choice (no shared state, no connection pooling concerns across unrelated validation targets — tests are short-lived).
- **`context.WithTimeout`** wraps the incoming `ctx` so the deadline applies to the dial+TLS+response.

```go
// Source: Go stdlib docs (HIGH confidence)
func (e *Executor) checkExternalConnectivity(ctx context.Context, target ConnectivityCheck) (bool, string) {
    timeout := target.TimeoutSeconds
    if timeout == 0 {
        timeout = DefaultConnectivityTimeoutSeconds
    }

    reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, target.URL, nil)
    if err != nil {
        return false, fmt.Sprintf("Invalid URL %s: %v", target.URL, err)
    }

    // EXT-02: override Host header for virtual-host routing
    if target.HostHeader != "" {
        req.Host = target.HostHeader
    }

    client := &http.Client{
        Timeout: time.Duration(timeout) * time.Second,
        CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
            return http.ErrUseLastResponse
        },
    }

    resp, err := client.Do(req)
    if err != nil {
        if target.ExpectedStatusCode == 0 {
            return true, fmt.Sprintf("Connection to %s blocked as expected", target.URL)
        }
        return false, fmt.Sprintf("Connection to %s failed: %v", target.URL, err)
    }
    defer func() { _ = resp.Body.Close() }()

    if resp.StatusCode == target.ExpectedStatusCode {
        return true, ""
    }
    return false, fmt.Sprintf("Connection to %s: got status %d, expected %d", target.URL, resp.StatusCode, target.ExpectedStatusCode)
}
```

**Note on `ExpectedStatusCode == 0` for external mode:** This case means "expect connection refused/timeout". A `net/http` client returns an error (not a response) when connection is refused or the dial times out — so the `err != nil` branch with the status-0 guard handles it correctly, matching the internal mode behavior.

### Pattern 5: Loader Parse-Time Validation

**What:** Reject the `mode: external` + `sourcePod` combination immediately at parse time in `parseSpec`.

```go
// Source: CONTEXT.md locked decision; follows existing validateSourcePod() pattern
case TypeConnectivity:
    var spec ConnectivitySpec
    if err := yaml.Unmarshal(specYAML, &spec); err != nil {
        return err
    }
    // EXT-01: fail fast if mode: external with sourcePod is incoherent
    if spec.Mode == "external" {
        sp := spec.SourcePod
        if sp.Name != "" || len(sp.LabelSelector) > 0 || sp.Namespace != "" {
            return fmt.Errorf("mode: external is incompatible with sourcePod (remove sourcePod or use mode: internal)")
        }
    } else if spec.Mode != "" && spec.Mode != "internal" {
        return fmt.Errorf("invalid mode %q: must be \"internal\" or \"external\"", spec.Mode)
    }
    if err := validateSourcePod(spec.SourcePod); err != nil {
        return err
    }
    // apply default timeout...
```

### Anti-Patterns to Avoid

- **Setting `req.Header.Set("Host", h)`:** This does NOT override the wire Host header in Go. Only `req.Host = h` works. (HIGH confidence — stdlib behavior).
- **Sharing a single `http.Client` across goroutines within `ExecuteAll`:** While `http.Client` is safe for concurrent use, connection pool reuse across unrelated targets can cause surprising behavior. Per-request clients are simpler and the overhead is negligible for validation calls.
- **Reading the response body:** For status-code-only validation, the body must still be drained or `resp.Body.Close()` called to prevent connection leaks. Use `defer resp.Body.Close()` always.
- **Calling `http.Get(url)`:** This uses the default global client with no timeout. Always use a client with timeout or `context.WithTimeout`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| DNS resolution for sslip.io | Custom resolver, /etc/hosts manipulation | `net/http` default resolver | sslip.io is public DNS — resolves 127-0-0-1 → 127.0.0.1 automatically |
| Host header override | Custom transport, raw TCP | `req.Host = h` | stdlib supports this natively |
| Redirect control | Counting redirects, parsing Location header | `http.ErrUseLastResponse` in `CheckRedirect` | stdlib mechanism, one line |
| Timeout enforcement | Manual goroutine + select | `context.WithTimeout` + client.Timeout | Established project pattern (Phases 6, 7) |

**Key insight:** The entire external HTTP check is ~20 lines of stdlib code. No new libraries.

---

## Common Pitfalls

### Pitfall 1: Host Header Set via Header Map (Not req.Host)

**What goes wrong:** `req.Header.Set("Host", "myapp.example.com")` — the Host header on the wire is still derived from the URL. The virtual-host routing in Ingress sees the wrong hostname.

**Why it happens:** Go's net/http treats `Host` as a special request property. The `req.Header` map is for general headers; `req.Host` is the canonical way.

**How to avoid:** Always use `req.Host = target.HostHeader` when a HostHeader override is needed.

**Warning signs:** Ingress returns 404 or routes to wrong backend despite `hostHeader` being set.

### Pitfall 2: No Response Body Drain/Close

**What goes wrong:** `resp.Body` not closed → connection kept open → resource leak in long-running processes.

**Why it happens:** `net/http` requires callers to close the body even when the body content is not needed.

**How to avoid:** `defer func() { _ = resp.Body.Close() }()` immediately after `resp, err := client.Do(req)`.

### Pitfall 3: Mode Field Empty String vs "internal"

**What goes wrong:** Existing connectivity specs in challenges have no `mode` field. After adding the `Mode` field, a spec with no `mode` key deserializes to `Mode == ""` (empty string, not `"internal"`). The branch condition `spec.Mode == "external"` must be the only special case — everything else (including empty) goes to the existing internal path.

**Why it happens:** Go YAML unmarshaling sets missing optional fields to their zero value (`""` for string).

**How to avoid:** In `executeConnectivity`, the branch is `if spec.Mode == "external" { ... }`, not `if spec.Mode == "internal" { ... }`. All existing tests continue to pass unchanged.

### Pitfall 4: Double-Timeout (Client.Timeout + context.WithTimeout)

**What goes wrong:** Both `http.Client{Timeout: t}` and `context.WithTimeout(ctx, t)` set — whichever fires first cancels the request. If the outer `ctx` has a shorter deadline (e.g., overall validation timeout), it will cancel before the per-target timeout.

**Why it happens:** Both mechanisms are active simultaneously.

**How to avoid:** Using `context.WithTimeout` with the incoming `ctx` means the per-target deadline is relative to the caller's context. Setting `client.Timeout` as well provides a hard ceiling independent of context. This is fine — the effective timeout is `min(client.Timeout, remaining ctx deadline)`. The implementation should set both to `timeout` seconds for clarity.

### Pitfall 5: macOS Docker Routing (sslip.io vs non-sslip.io)

**What goes wrong:** If a challenge author uses a hard-coded `http://192.168.x.x:8080/` IP (a cloud-provider-kind LoadBalancer IP), that IP is only routable from inside the Kind network on macOS — the CLI host cannot reach it.

**Why it happens:** cloud-provider-kind assigns IPs in the Docker bridge network (172.x.x.x range on macOS), which is not routable from the macOS host.

**How to avoid:** Challenge authors MUST use sslip.io hostnames encoding 127.0.0.1 (e.g., `http://myapp.127-0-0-1.sslip.io:8080/`) for `mode: external` checks on macOS. The CLI does NOT need to handle this case specially — it is a challenge authoring constraint. Consider documenting this in a challenge authoring guide.

**Warning signs:** External check returns "connection refused" on macOS while working in CI (Linux Docker where bridge IPs are routable).

---

## Code Examples

Verified patterns from Go stdlib and existing codebase:

### External HTTP Check — Complete Method

```go
// Source: Go stdlib net/http docs + CONTEXT.md patterns
func (e *Executor) checkExternalConnectivity(ctx context.Context, target ConnectivityCheck) (bool, string) {
    timeout := target.TimeoutSeconds
    if timeout == 0 {
        timeout = DefaultConnectivityTimeoutSeconds
    }

    reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, target.URL, nil)
    if err != nil {
        return false, fmt.Sprintf("Invalid URL %s: %v", target.URL, err)
    }

    // EXT-02: hostHeader overrides the wire Host header for virtual-host routing
    if target.HostHeader != "" {
        req.Host = target.HostHeader
    }

    client := &http.Client{
        Timeout: time.Duration(timeout) * time.Second,
        CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
            return http.ErrUseLastResponse
        },
    }

    resp, err := client.Do(req)
    if err != nil {
        // Connection refused or timeout counts as "blocked" for ExpectedStatusCode==0
        if target.ExpectedStatusCode == 0 {
            return true, fmt.Sprintf("Connection to %s blocked as expected", target.URL)
        }
        return false, fmt.Sprintf("Connection to %s failed: %v", target.URL, err)
    }
    defer func() { _ = resp.Body.Close() }()

    if resp.StatusCode == target.ExpectedStatusCode {
        return true, ""
    }
    return false, fmt.Sprintf("Connection to %s: got status %d, expected %d",
        target.URL, resp.StatusCode, target.ExpectedStatusCode)
}
```

### checkExternalConnectivityAll Helper

```go
// Iterates targets, collects failures — mirrors checkConnectivity loop in executeConnectivity
func (e *Executor) checkExternalConnectivityAll(ctx context.Context, spec ConnectivitySpec) (bool, string, error) {
    allPassed := true
    var messages []string
    for _, target := range spec.Targets {
        passed, msg := e.checkExternalConnectivity(ctx, target)
        if !passed {
            allPassed = false
            messages = append(messages, msg)
        }
    }
    if allPassed {
        return true, msgAllConnectivityPassed, nil
    }
    return false, strings.Join(messages, "; "), nil
}
```

### sslip.io Challenge YAML Example

```yaml
# Source: EXT-03 requirement
- key: ingress-reachable
  title: "Ingress Accessible"
  type: connectivity
  spec:
    mode: external
    targets:
      - url: http://myapp.127-0-0-1.sslip.io:8080/
        expectedStatusCode: 200
        timeoutSeconds: 10
```

### hostHeader Example (direct IP URL)

```yaml
# Source: EXT-02 requirement
- key: ingress-virtual-host
  title: "Ingress Virtual Host"
  type: connectivity
  spec:
    mode: external
    targets:
      - url: http://127.0.0.1:8080/
        hostHeader: myapp.example.com
        expectedStatusCode: 200
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Connectivity checks: pod exec only | Connectivity checks: pod exec OR host net/http (mode field) | Phase 8 | Enables Ingress/Gateway external validation without a curl pod |
| Ingress testing: requires in-cluster curl pod | Ingress testing: CLI sends request directly via sslip.io | Phase 8 | Simpler challenge authoring, no probe pod overhead for external checks |

---

## Open Questions

1. **`net/http` import in executor.go**
   - What we know: `net/http` is NOT currently imported in `executor.go` (only in `loader.go`).
   - What's unclear: nothing — just needs to be added to the import block in executor.go.
   - Recommendation: Add `"net/http"` and `"time"` to executor.go imports (time is already imported).

2. **Test environment guard for external mode**
   - What we know: `checkConnectivity` has a `e.restConfig.Host == ""` guard for fake clients. External mode (`checkExternalConnectivity`) uses `net/http` against real URLs — no K8s client involved.
   - What's unclear: In unit tests, `http://...` URLs will fail to connect (no server). Should we guard or let tests use `httptest.NewServer`?
   - Recommendation: Unit tests for `checkExternalConnectivity` should use `httptest.NewServer` to control responses. No special test-environment guard needed — `net/http` failure is deterministic and testable.

3. **sslip.io availability in CI**
   - What we know: sslip.io is a public DNS service. CI runners need external DNS resolution.
   - What's unclear: Integration tests that actually fire HTTP requests to sslip.io URLs.
   - Recommendation: Unit tests use `httptest.NewServer` with localhost URLs, not sslip.io. No DNS dependency in tests.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.x (existing) |
| Config file | Taskfile.yml (`task test:unit`) |
| Quick run command | `go test ./internal/validation/... -run TestExternal -v` |
| Full suite command | `task test:unit` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| EXT-01 | `mode: external` parses successfully; `executeConnectivity` routes to net/http path | unit | `go test ./internal/validation/... -run TestParse_Connectivity_ExternalMode -v` | Wave 0 |
| EXT-01 | `mode: external` + `sourcePod` returns parse error | unit | `go test ./internal/validation/... -run TestParse_Connectivity_ExternalModeWithSourcePod -v` | Wave 0 |
| EXT-01 | `checkExternalConnectivity` returns passed=true when server responds with expected status | unit | `go test ./internal/validation/... -run TestCheckExternalConnectivity_Success -v` | Wave 0 |
| EXT-01 | `checkExternalConnectivity` returns passed=false when server responds with wrong status | unit | `go test ./internal/validation/... -run TestCheckExternalConnectivity_WrongStatus -v` | Wave 0 |
| EXT-01 | `checkExternalConnectivity` returns passed=true when connection refused + ExpectedStatusCode==0 | unit | `go test ./internal/validation/... -run TestCheckExternalConnectivity_BlockedConnection -v` | Wave 0 |
| EXT-01 | `checkExternalConnectivity` returns passed=false when connection refused + ExpectedStatusCode!=0 | unit | `go test ./internal/validation/... -run TestCheckExternalConnectivity_ConnectionRefused -v` | Wave 0 |
| EXT-02 | `hostHeader` sets `req.Host` on the outgoing request | unit | `go test ./internal/validation/... -run TestCheckExternalConnectivity_HostHeader -v` | Wave 0 |
| EXT-03 | sslip.io URL parses and is sent as-is (no special resolution) | unit | `go test ./internal/validation/... -run TestParse_Connectivity_SslipIO -v` | Wave 0 |
| EXT-04 | Status code comparison works for all common codes (200, 201, 301, 404) | unit | `go test ./internal/validation/... -run TestCheckExternalConnectivity_StatusCodes -v` | Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./internal/validation/... -count=1`
- **Per wave merge:** `task test:unit`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

All test functions listed above are new — they do not exist yet. They must be added to `executor_test.go` and `loader_test.go`.

- [ ] `TestParse_Connectivity_ExternalMode` in `loader_test.go` — covers EXT-01 parse path
- [ ] `TestParse_Connectivity_ExternalModeWithSourcePod` in `loader_test.go` — covers EXT-01 error
- [ ] `TestCheckExternalConnectivity_Success` in `executor_test.go` — covers EXT-01, EXT-04 (uses `httptest.NewServer`)
- [ ] `TestCheckExternalConnectivity_WrongStatus` in `executor_test.go` — covers EXT-04
- [ ] `TestCheckExternalConnectivity_BlockedConnection` in `executor_test.go` — covers EXT-01 status-0 guard
- [ ] `TestCheckExternalConnectivity_ConnectionRefused` in `executor_test.go` — covers EXT-01 failure path
- [ ] `TestCheckExternalConnectivity_HostHeader` in `executor_test.go` — covers EXT-02 (assert req.Host via httptest server reading `r.Host`)
- [ ] `TestParse_Connectivity_SslipIO` in `loader_test.go` — covers EXT-03 (parse only, no DNS call)

---

## Sources

### Primary (HIGH confidence)

- Go stdlib `net/http` package documentation — `req.Host`, `CheckRedirect`, `http.ErrUseLastResponse`, `http.NewRequestWithContext`
- Go stdlib `context` package — `context.WithTimeout` propagation
- Existing `internal/validation/executor.go` — established patterns (restConfig guard, context.WithTimeout in probe, checkConnectivity structure)
- Existing `internal/validation/types.go` — `ConnectivitySpec`, `ConnectivityCheck` struct layout
- Existing `internal/validation/loader.go` — `parseSpec` switch, `validateSourcePod` pattern

### Secondary (MEDIUM confidence)

- sslip.io public documentation — DNS wildcard resolving embedded IPs to real IPs (e.g., `myapp.127-0-0-1.sslip.io` → A record 127.0.0.1)
- CONTEXT.md phase 08 — locked implementation decisions (considered authoritative for this project)

### Tertiary (LOW confidence)

- Phase 8 concern in STATE.md: "macOS Docker IP reachability with cloud-provider-kind v0.10.0 is MEDIUM confidence" — this is about the non-sslip.io path which is out of scope for Phase 8.

---

## Metadata

**Confidence breakdown:**

- Standard stack: HIGH — stdlib only, no new deps
- Architecture: HIGH — locked decisions in CONTEXT.md + direct code inspection of files being modified
- Pitfalls: HIGH — `req.Host` vs header map is a well-known Go gotcha; body close is standard; mode empty-string behavior is from direct YAML unmarshaling inspection
- Test patterns: HIGH — `httptest.NewServer` is the standard Go way to unit-test net/http callers

**Research date:** 2026-03-11
**Valid until:** 2026-06-11 (stdlib patterns are stable; no external dependencies to version-drift)
