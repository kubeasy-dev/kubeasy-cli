# Phase 5: Security Hardening - Context

**Gathered:** 2026-03-11
**Status:** Ready for planning

<domain>
## Phase Boundary

Fix two concrete attack surfaces in the CLI:
1. Shell injection in `checkConnectivity` — replace `sh -c` curl invocation with direct exec args
2. Arbitrary HTTP fetch in `FetchManifest` — add a domain allowlist before fetching

No new validation types, no architectural changes. Scope: `internal/validation/executor.go` and `internal/kube/manifest.go`.

</domain>

<decisions>
## Implementation Decisions

### SEC-01: Shell injection (executeConnectivity)
- Fix the primary curl path: replace `sh -c "curl ... 'URL'"` with direct args `["curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", "--connect-timeout", "<N>", url]`
- The `escapeShellArg` helper and `sh -c` wrapper are removed from the curl invocation
- **Wget fallback**: leave as-is for now — revisit in a later phase when the validation approach is re-examined
- SEC-01 is satisfied by fixing the primary (curl) path; the fallback is a deferred improvement

### SEC-02: FetchManifest allowlist
- Add a domain-level allowlist inside `FetchManifest` — no signature change, no impact on callers
- Trusted domains: `https://github.com/` and `https://raw.githubusercontent.com/`
- Pattern mirrors `loadFromURL` in `loader.go` (validates against `ChallengesRepoBaseURL` before fetching)
- If URL does not match any allowed prefix: return an error, do not fetch
- Remove the `#nosec G107` comment once the allowlist is in place

### Claude's Discretion
- Exact error message format for rejected URLs
- Whether to extract the allowlist into a package-level variable or keep it inline
- Test structure for the allowlist validation

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `loadFromURL` in `loader.go`: established pattern — `strings.HasPrefix(url, trustedBase)` before `http.Get` — apply same to `FetchManifest`
- `escapeShellArg` in `executor.go`: no longer needed once curl uses direct args — can be removed

### Established Patterns
- Direct args in `cmd []string{}` slices already used throughout the codebase (e.g., Kubernetes exec options)
- `strings.HasPrefix` URL validation: used in `loadFromURL`, consistent and idiomatic

### Integration Points
- `checkConnectivity` in `executor.go`: change `cmd` slice construction for curl only; wget fallback unchanged
- `FetchManifest` in `kube/manifest.go`: add prefix check before `http.Get`; callers in `deployer/infrastructure.go` unaffected (their URLs already start with allowed prefixes)

</code_context>

<specifics>
## Specific Ideas

- The wget fallback uses `sh -c` for `echo 200 || echo 000` — this is acknowledged as a deferred improvement, not in scope for Phase 5
- Domain-level allowlist (not path-level) means future manifests from GitHub repos won't require allowlist updates

</specifics>

<deferred>
## Deferred Ideas

- Wget fallback shell injection fix — user requested to revisit the connectivity validation approach in a later phase rather than patch the fallback now

</deferred>

---

*Phase: 05-security-hardening*
*Context gathered: 2026-03-11*
