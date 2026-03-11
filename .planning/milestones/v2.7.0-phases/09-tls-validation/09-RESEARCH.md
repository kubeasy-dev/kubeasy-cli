# Phase 9: TLS Validation - Research

**Researched:** 2026-03-11
**Domain:** Go standard library TLS — `crypto/tls`, `net/http`, `net/http/httptest`
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- New `tls:` sub-block on `ConnectivityCheck` (per-target, not spec-level) — consistent with `hostHeader` placement
- Fields: `insecureSkipVerify: bool`, `validateExpiry: bool`, `validateSANs: bool`
- The `tls:` block is optional — omitting it means no explicit TLS checks are requested
- Explicit opt-in: TLS-01 and TLS-02 checks only run when `validateExpiry: true` or `validateSANs: true`
- Without a `tls:` block, `https://` still works (Go's standard TLS validates automatically, raw errors)
- `insecureSkipVerify: true` skips ALL cert checks: CA trust, expiry, AND hostname SANs
- `insecureSkipVerify: true` takes priority over `validateExpiry`/`validateSANs` — no manual cert inspection runs
- Friendly + cert metadata messages when `validateExpiry` or `validateSANs` is true:
  - Expiry failure: `"Certificate expired on 2025-01-01 (2 months ago)"`
  - SAN mismatch: `"Hostname 'myapp.example.com' not in SANs: [myapp.sslip.io, *.sslip.io]"`
- TLS failure must short-circuit the HTTP status code check — never report `"got status 0, expected 200"` when TLS is the actual failure

### Claude's Discretion
- Whether to use a separate `tls.Dial` probe to fetch cert metadata for friendly messages vs parsing Go TLS error strings
- Whether `TLSConfig` is a struct pointer (`*TLSConfig`) or inline fields on `ConnectivityCheck`
- Test strategy for TLS: `httptest.TLSServer` vs custom `crypto/tls` cert generation

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| TLS-01 | External check validates that TLS cert is not expired (`NotAfter > now`) | `resp.TLS.PeerCertificates[0].NotAfter` available post-handshake; separate `tls.Dial` probe gives cert even when HTTP fails |
| TLS-02 | External check validates hostname matches cert SANs (`DNSNames`) | `resp.TLS.PeerCertificates[0].DNSNames` + `IPAddresses`; `x509.Certificate.VerifyHostname()` for correct wildcard matching |
| TLS-03 | `ConnectivityCheck` supports `insecureSkipVerify: true` for self-signed certs in Kind | `tls.Config{InsecureSkipVerify: true}` on `http.Transport`; `httptest.NewTLSServer` sufficient for test |
</phase_requirements>

---

## Summary

Phase 9 is a pure Go standard library feature addition — no new dependencies required. The entire implementation lives in `internal/validation/types.go` (new `TLSConfig` struct + field on `ConnectivityCheck`) and `internal/validation/executor.go` (`checkExternalConnectivity` modified to build a custom `tls.Config` and, when needed, inspect `resp.TLS.PeerCertificates[0]`).

The key design question (Claude's discretion) is how to get cert metadata when `validateExpiry` or `validateSANs` is true. If the HTTPS request succeeds, `resp.TLS` contains the peer certificate. But if the TLS handshake fails (e.g., cert expired and Go's default verification rejects it), `resp` is nil — so cert metadata must come from a separate `tls.Dial` probe executed first. The probe approach is cleaner and more reliable than parsing Go error strings, which are not part of the public API and could change between Go versions.

The `insecureSkipVerify: true` case is simpler: set `tls.Config{InsecureSkipVerify: true}` on the transport, make the HTTP request, no cert inspection needed. For explicit `validateExpiry`/`validateSANs` checks, the flow is: (1) dial with `InsecureSkipVerify: true` to obtain the raw cert, (2) inspect `NotAfter`/`DNSNames` manually, (3) if check passes proceed with the actual HTTP request (which may use `InsecureSkipVerify` or not depending on config).

**Primary recommendation:** Use a separate `tls.Dial` probe (with `InsecureSkipVerify: true` to always succeed) to extract cert metadata, then run explicit checks. This avoids parsing error strings and works regardless of whether the cert is valid from Go's CA-trust perspective.

---

## Standard Stack

### Core (all stdlib — no new dependencies)

| Package | Purpose | Notes |
|---------|---------|-------|
| `crypto/tls` | TLS config, cert types, `tls.Dial` | `tls.Config{InsecureSkipVerify}`, `tls.ConnectionState.PeerCertificates` |
| `crypto/x509` | Certificate type, `VerifyHostname` | `x509.Certificate.NotAfter`, `DNSNames`, `VerifyHostname(host)` |
| `net/http` | HTTP client with custom transport | `&http.Transport{TLSClientConfig: &tls.Config{...}}` |
| `net/http/httptest` | TLS test server | `httptest.NewTLSServer(handler)` — uses self-signed cert, no cert-manager needed |
| `time` | Expiry delta computation | `time.Since(cert.NotAfter)` for human-readable message |

### No New Dependencies
Go 1.25.4 (project's version) ships all required packages in stdlib. No `go get` needed.

---

## Architecture Patterns

### Recommended Change Surface

```
internal/validation/
├── types.go        # Add TLSConfig struct; add TLS *TLSConfig field to ConnectivityCheck
├── executor.go     # Modify checkExternalConnectivity — build tls.Config, probe cert, friendly errors
└── loader.go       # Optional: warn/log when insecureSkipVerify + validateExpiry both true (no error — insecureSkipVerify wins)
```

### Pattern 1: TLSConfig struct (pointer on ConnectivityCheck)

Using a pointer (`*TLSConfig`) means the zero value of `ConnectivityCheck` has `nil` TLS config, which maps cleanly to "no explicit TLS checks" without needing to distinguish absent vs zero-value bool fields.

```go
// TLSConfig controls TLS validation behaviour for a single external connectivity check.
// Optional — omitting it leaves Go's default TLS verification in place.
type TLSConfig struct {
    // InsecureSkipVerify skips ALL certificate checks (CA trust, expiry, SANs).
    // Primary use: cert-manager self-signed certs in Kind clusters.
    // Takes priority over ValidateExpiry and ValidateSANs when true.
    InsecureSkipVerify bool `yaml:"insecureSkipVerify,omitempty" json:"insecureSkipVerify,omitempty"`

    // ValidateExpiry explicitly checks cert NotAfter > now and returns a friendly message.
    // Only active when InsecureSkipVerify is false.
    ValidateExpiry bool `yaml:"validateExpiry,omitempty" json:"validateExpiry,omitempty"`

    // ValidateSANs explicitly checks that the request hostname appears in cert DNSNames/IPAddresses.
    // Only active when InsecureSkipVerify is false.
    ValidateSANs bool `yaml:"validateSANs,omitempty" json:"validateSANs,omitempty"`
}

// In ConnectivityCheck — add after HostHeader:
// TLS configures TLS validation for external https:// checks.
// Optional — absent means no explicit TLS checks.
TLS *TLSConfig `yaml:"tls,omitempty" json:"tls,omitempty"`
```

### Pattern 2: Building the http.Client with TLS config

```go
// Source: Go stdlib net/http + crypto/tls docs
tlsCfg := &tls.Config{}
if target.TLS != nil && target.TLS.InsecureSkipVerify {
    tlsCfg.InsecureSkipVerify = true //nolint:gosec // intentional: self-signed certs in Kind
}

client := &http.Client{
    Timeout: time.Duration(timeout) * time.Second,
    Transport: &http.Transport{
        TLSClientConfig: tlsCfg,
    },
    CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
        return http.ErrUseLastResponse
    },
}
```

### Pattern 3: Fetching cert metadata via tls.Dial probe

When `validateExpiry: true` or `validateSANs: true` is set (and `insecureSkipVerify` is false or absent), fetch the cert separately:

```go
// Source: Go stdlib crypto/tls docs
// Parse host:port from URL for tls.Dial
u, _ := url.Parse(target.URL)
host := u.Hostname()
port := u.Port()
if port == "" {
    port = "443"
}

// Always use InsecureSkipVerify in the probe so we get cert metadata
// even when cert is expired or uses untrusted CA
conn, err := tls.Dial("tcp", net.JoinHostPort(host, port), &tls.Config{
    InsecureSkipVerify: true, //nolint:gosec // probe only — we validate manually below
})
if err != nil {
    return false, fmt.Sprintf("TLS dial failed: %v", err)
}
_ = conn.Close()

cert := conn.ConnectionState().PeerCertificates[0]
```

### Pattern 4: Expiry check (TLS-01)

```go
// Source: crypto/x509 — x509.Certificate.NotAfter is time.Time
if target.TLS.ValidateExpiry && !target.TLS.InsecureSkipVerify {
    if time.Now().After(cert.NotAfter) {
        delta := time.Since(cert.NotAfter)
        months := int(delta.Hours() / 24 / 30)
        return false, fmt.Sprintf("Certificate expired on %s (%d months ago)",
            cert.NotAfter.Format("2006-01-02"), months)
    }
}
```

### Pattern 5: SAN hostname check (TLS-02)

```go
// Source: crypto/x509 — VerifyHostname handles wildcards correctly
if target.TLS.ValidateSANs && !target.TLS.InsecureSkipVerify {
    u, _ := url.Parse(target.URL)
    hostname := u.Hostname()
    // Use HostHeader if set — that's the logical hostname for SAN matching
    if target.HostHeader != "" {
        hostname = target.HostHeader
    }
    if err := cert.VerifyHostname(hostname); err != nil {
        return false, fmt.Sprintf("Hostname %q not in SANs: %v",
            hostname, cert.DNSNames)
    }
}
```

**IMPORTANT:** `VerifyHostname` handles wildcard SANs (`*.example.com`) correctly; manual string matching does not. Always use `cert.VerifyHostname(hostname)`.

### Pattern 6: Short-circuit on TLS failure

TLS failures must not leak a confusing "got status 0, expected 200" message. Structure the function to return early on TLS failures before issuing the HTTP request:

```go
func (e *Executor) checkExternalConnectivity(ctx context.Context, target ConnectivityCheck) (bool, string) {
    // Step 1: Build TLS config
    // Step 2: If explicit TLS checks requested, probe cert and validate
    //         Return false + friendly message if check fails — do NOT proceed to HTTP
    // Step 3: Issue HTTP request with the configured tls.Config
    // Step 4: Check status code
}
```

### Anti-Patterns to Avoid

- **Parsing Go TLS error strings:** `x509.CertificateInvalidError` error messages are not stable API. Use `tls.Dial` probe + direct struct field access instead.
- **Using resp.TLS for manual checks when validation fails:** `resp` is nil when the TLS handshake itself fails. Always probe separately.
- **String-matching SANs manually:** Wildcard certs (`*.svc.cluster.local`) require proper wildcard matching. Use `cert.VerifyHostname()`.
- **Exposing `InsecureSkipVerify` without `//nolint:gosec`:** gosec G402 flags this. Add the nolint directive with a comment explaining it's intentional.
- **Forgetting HostHeader vs URL hostname for SAN check:** When `hostHeader` is set (virtual-host routing), the SAN check should use the hostHeader hostname, not the URL hostname (which may be an IP or sslip.io address).

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Wildcard SAN matching | Custom `strings.HasPrefix` logic | `cert.VerifyHostname(host)` | Wildcards have complex rules (only first label, no nested wildcards); stdlib handles them correctly |
| Cert chain parsing | Manual ASN.1 DER decoding | `tls.Dial` → `conn.ConnectionState().PeerCertificates` | Already parsed as `*x509.Certificate` |
| TLS test server with self-signed cert | Custom cert generation with `crypto/rand` | `httptest.NewTLSServer(handler)` | Built-in, uses pre-generated self-signed cert, zero setup |

**Key insight:** This entire feature is covered by Go's stdlib. No new packages, no cert-manager integration in tests, no custom cert generation unless testing expiry scenarios specifically.

---

## Common Pitfalls

### Pitfall 1: Nil resp when TLS handshake fails
**What goes wrong:** Code calls `resp.TLS.PeerCertificates[0]` after `client.Do(req)` but TLS handshake failure returns `nil, err` — panics.
**Why it happens:** Confusing "we have a response" with "TLS succeeded". When cert validation fails (expired, CA not trusted), `client.Do` returns `nil, err`.
**How to avoid:** Use `tls.Dial` probe to get the cert before the HTTP request. Never read `resp.TLS` without first checking `resp != nil`.
**Warning signs:** Any code path that reads `resp.TLS` after a potentially-failing `client.Do`.

### Pitfall 2: gosec G402 linting error
**What goes wrong:** `tls.Config{InsecureSkipVerify: true}` triggers gosec G402 ("TLS InsecureSkipVerify set to true").
**Why it happens:** gosec flags this as a security risk; CI runs golangci-lint which includes gosec.
**How to avoid:** Add `//nolint:gosec // intentional: self-signed certs in Kind cluster` on the `InsecureSkipVerify: true` assignment line. The project already does this pattern (see `internal/kube/`).
**Warning signs:** Lint failure in CI with "G402" code.

### Pitfall 3: HostHeader vs URL hostname for SAN validation
**What goes wrong:** SAN check uses URL host (e.g., `127-0-0-1.sslip.io`) but the cert SANs contain the service hostname (e.g., `myapp.example.com` set via hostHeader).
**Why it happens:** External checks can route to a real IP/sslip.io URL but set a virtual-host HostHeader — the cert is issued for the hostname, not the IP.
**How to avoid:** SAN validation should use `target.HostHeader` when set, falling back to `url.Parse(target.URL).Hostname()`.
**Warning signs:** SAN check fails for valid certs on Ingress-routed HTTPS endpoints.

### Pitfall 4: tls.Dial probe context timeout
**What goes wrong:** `tls.Dial` does not accept a `context.Context` directly (it uses the net package's `DialContext` underneath), so the per-target timeout may not be properly applied.
**How to avoid:** Use `(&tls.Dialer{Config: tlsCfg}).DialContext(ctx, "tcp", addr)` which accepts a context and honors deadlines.
**Reference:** Go 1.15+ — `tls.Dialer` struct with `DialContext` method.

```go
// Source: Go stdlib crypto/tls — tls.Dialer (Go 1.15+)
dialer := &tls.Dialer{
    Config: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
}
conn, err := dialer.DialContext(reqCtx, "tcp", net.JoinHostPort(host, port))
```

### Pitfall 5: httptest.TLSServer certificate details
**What goes wrong:** `httptest.NewTLSServer` uses a fixed test cert with a specific expiry and SANs — not suitable for testing expiry-failure cases.
**Why it happens:** The built-in httptest cert has a long expiry (years) and only `127.0.0.1` as SAN.
**How to avoid:** For testing expiry failures (TLS-01 RED case), generate a custom expired cert using `crypto/x509` + `crypto/rsa` in the test itself. For success cases and TLS-03 (`insecureSkipVerify`), `httptest.NewTLSServer` is sufficient.
**Note:** The `httptest.Server.TLS` field exposes the `*tls.Config` including the test certificate.

---

## Code Examples

### Example: Custom expired cert for test

```go
// Source: crypto/x509 + crypto/rsa stdlib
// For TLS-01 test: expired cert
func generateExpiredCert(t *testing.T) tls.Certificate {
    t.Helper()
    key, _ := rsa.GenerateKey(rand.Reader, 2048)
    template := &x509.Certificate{
        SerialNumber: big.NewInt(1),
        Subject:      pkix.Name{CommonName: "expired.example.com"},
        NotBefore:    time.Now().Add(-48 * time.Hour),
        NotAfter:     time.Now().Add(-24 * time.Hour), // already expired
        DNSNames:     []string{"expired.example.com"},
    }
    certDER, _ := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
    certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
    keyPEM, _ := x509.MarshalPKCS8PrivateKey(key)
    keyPEMBlock := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyPEM})
    cert, _ := tls.X509KeyPair(certPEM, keyPEMBlock)
    return cert
}
```

### Example: httptest.NewUnstartedServer with custom TLS cert

```go
// Source: net/http/httptest stdlib
srv := httptest.NewUnstartedServer(handler)
srv.TLS = &tls.Config{
    Certificates: []tls.Certificate{generateExpiredCert(t)},
}
srv.StartTLS()
defer srv.Close()
// srv.URL is now https://127.0.0.1:<port>
```

### Example: InsecureSkipVerify for httptest TLS server

```go
// Source: net/http/httptest — TLSServer uses self-signed cert not in OS trust store
// httptest.Server.Client() returns a pre-configured client that trusts the test cert
// But for testing our own code, we set InsecureSkipVerify on the target:
target := ConnectivityCheck{
    URL:                srv.URL + "/",
    ExpectedStatusCode: 200,
    TLS: &TLSConfig{InsecureSkipVerify: true},
}
```

### Example: SAN check using VerifyHostname

```go
// Source: crypto/x509 stdlib
err := cert.VerifyHostname("myapp.example.com")
// err == nil: hostname is in SANs (handles wildcards correctly)
// err != nil: hostname not in SANs — use cert.DNSNames for the error message
```

---

## State of the Art

| Old Approach | Current Approach | Notes |
|--------------|-----------------|-------|
| Manual TLS error string parsing | `tls.Dial` probe + direct cert field access | Error strings not part of Go public API |
| `net.Dial` + manual TLS upgrade | `tls.Dialer.DialContext` (Go 1.15+) | Context-aware, cleaner API |
| Manual wildcard matching | `cert.VerifyHostname(host)` | Handles edge cases correctly |

**Deprecated/outdated:**
- `tls.Dial` (the standalone function): still works but does not accept a context. Prefer `tls.Dialer.DialContext` for proper timeout propagation.

---

## Open Questions

1. **Delta formatting in expiry message**
   - What we know: Decision says `"Certificate expired on 2025-01-01 (2 months ago)"`
   - What's unclear: Exact rounding — use months, days, or both? e.g., "47 days ago" vs "1 month ago"
   - Recommendation: Use days for precision under 60 days, months beyond that. Or simply always use days — simpler, no ambiguity.

2. **httptest.TLSServer for SAN mismatch test**
   - What we know: `httptest.NewTLSServer` cert has `127.0.0.1` as SAN, no DNS names
   - What's unclear: Can we use it to test SAN mismatch by setting `validateSANs: true` with a hostname that is NOT `127.0.0.1`?
   - Recommendation: Yes — set `hostHeader: "myapp.example.com"` + `validateSANs: true`; cert has only `127.0.0.1` SAN so check will fail. No custom cert generation needed for TLS-02 failure test.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` + `testify` v1.11.1 |
| Config file | none — standard `go test ./...` |
| Quick run command | `task test:unit` |
| Full suite command | `task test` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TLS-01 | Expired cert returns friendly "Certificate expired on..." message | unit | `task test:unit` (executor_test.go) | Wave 0 |
| TLS-01 | Valid cert with `validateExpiry: true` passes | unit | `task test:unit` (executor_test.go) | Wave 0 |
| TLS-02 | Hostname not in SANs returns friendly message with SAN list | unit | `task test:unit` (executor_test.go) | Wave 0 |
| TLS-02 | Hostname matches SANs passes | unit | `task test:unit` (executor_test.go) | Wave 0 |
| TLS-03 | Self-signed cert + `insecureSkipVerify: true` succeeds | unit | `task test:unit` (executor_test.go) | Wave 0 |
| TLS-03 | `tls:` block parses correctly from YAML | unit | `task test:unit` (loader_test.go) | Wave 0 |

### Sampling Rate
- **Per task commit:** `task test:unit`
- **Per wave merge:** `task test`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] New test functions in `internal/validation/executor_test.go` — covers TLS-01, TLS-02, TLS-03 (file exists, append new test functions)
- [ ] New test functions in `internal/validation/loader_test.go` — covers `tls:` YAML parsing (file exists, append)
- No new framework or config needed

---

## Sources

### Primary (HIGH confidence)
- Go stdlib `crypto/tls` — `tls.Config`, `tls.Dialer`, `tls.ConnectionState`, `PeerCertificates`
- Go stdlib `crypto/x509` — `x509.Certificate.NotAfter`, `DNSNames`, `VerifyHostname`
- Go stdlib `net/http/httptest` — `NewTLSServer`, `NewUnstartedServer`, `Server.TLS`
- Existing codebase: `internal/validation/executor.go`, `types.go`, `loader.go` — read directly

### Secondary (MEDIUM confidence)
- `tls.Dialer.DialContext` — Go 1.15+ API, well-established pattern for context-aware TLS dial

### Tertiary (LOW confidence)
None.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — pure stdlib, no external packages, code read directly
- Architecture: HIGH — patterns derived from existing codebase conventions and stdlib docs
- Pitfalls: HIGH — gosec G402 pattern already present in project; httptest behavior verified from stdlib

**Research date:** 2026-03-11
**Valid until:** Stable (stdlib patterns; valid until Go major version change)
