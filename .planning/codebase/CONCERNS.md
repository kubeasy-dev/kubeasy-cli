# Codebase Concerns

**Analysis Date:** 2026-03-09

---

## Tech Debt

**Hardcoded developer machine path in loader.go:**
- Issue: `FindLocalChallengeFile` includes `filepath.Join(os.Getenv("HOME"), "Workspace", "kubeasy", "challenges", slug, "challenge.yaml")` as a fallback path — this is specific to the developer's local layout and will silently pick up a local file on any machine where that path happens to exist.
- Files: `internal/validation/loader.go:52`
- Impact: A user's environment that coincidentally has that directory structure will load a local (possibly stale or wrong) challenge definition instead of fetching from GitHub, producing silent incorrect validation results.
- Fix approach: Remove the developer-specific path fallback. Local development should use an explicit `--challenge-dir` flag or env var instead.

**`WebsiteURL` defaults to localhost:3000:**
- Issue: `internal/constants/const.go` declares `var WebsiteURL = "http://localhost:3000"`. The correct production URL is injected only at release time via `ldflags` in `.goreleaser.yaml`. Any build that does not go through GoReleaser (local `go build`, `task build`) silently targets localhost and all API calls fail with connection refused.
- Files: `internal/constants/const.go:6`, `.goreleaser.yaml:25`
- Impact: Developers building locally without `task build` or without GoReleaser hit a confusing no-error-but-nothing-works failure mode. There is also a risk that a release build with a misconfigured pipeline ships with the localhost URL.
- Fix approach: Use a dedicated environment variable (`KUBEASY_API_URL`) as a fallback so local `go run` can point at a real backend without a full release pipeline, or set a sensible prod default in the constant.

**`ApplyManifest` swallows all manifest application errors:**
- Issue: `kube/manifest.go` returns `nil` unconditionally even when individual document apply/update operations fail. All failures are logged as warnings only.
- Files: `internal/kube/manifest.go:145`
- Impact: A manifest that silently fails to apply (e.g., a Kyverno policy that wasn't installed) leaves the challenge in a partially broken state. The command that triggered the deploy reports success, the user is confused when the challenge doesn't behave as expected.
- Fix approach: Collect errors from critical documents (non-optional resources) and return a combined error. Consider classifying documents or adding a "strict mode" parameter.

**Unchecked type assertions on `v.Spec` in executor:**
- Issue: `internal/validation/executor.go` uses bare type assertions (`v.Spec.(StatusSpec)`, `v.Spec.(ConditionSpec)`, etc.) without the comma-ok form. If a validation's spec fails to parse correctly at load time but a nil `Spec` slips through, the executor will panic.
- Files: `internal/validation/executor.go:73-85`
- Impact: A challenge YAML with a subtle spec error that passes `parseSpec` but leaves `Spec` as the wrong type will crash the entire CLI process instead of returning a graceful error for that validation.
- Fix approach: Use comma-ok assertions and return a `Result` with `Passed: false` and a descriptive error message instead of panicking.

**Duplicate manifest-walking logic:**
- Issue: `internal/deployer/challenge.go` and `internal/deployer/local.go` share nearly identical code blocks for walking `manifests/` and `policies/` subdirectories and applying YAML files. The logic is copy-pasted with only the source (OCI temp dir vs local dir) differing.
- Files: `internal/deployer/challenge.go:48-81`, `internal/deployer/local.go:29-62`
- Impact: Any fix or enhancement to manifest application logic (e.g., sorting order, error handling) must be applied in both places.
- Fix approach: Extract the walk-and-apply loop into a shared helper function in `internal/deployer/` that accepts a base directory path.

**Backward-compatibility wrapper proliferation in `internal/api/client.go`:**
- Issue: Multiple functions exist solely as backward-compatibility aliases: `GetChallenge` wraps `GetChallengeBySlug`, `GetChallengeProgress` wraps `GetChallengeStatus`, `ResetChallengeProgress` wraps `ResetChallenge`, `GetUserProfile` wraps `GetProfile`, `StartChallenge` wraps `StartChallengeWithResponse`, and `SendSubmit` wraps `SubmitChallenge`. This doubles the public API surface of the package.
- Files: `internal/api/client.go:170-174`, `internal/api/client.go:222-229`, `internal/api/client.go:232-251`, `internal/api/client.go:366-378`, `internal/api/client.go:76-79`
- Impact: Hard to know which function to call when writing new code. Maintaining two sets of names for the same operations adds confusion and maintenance burden.
- Fix approach: Pick one naming convention and update all callers to use only the primary names, then delete the aliases.

**Polling loops without exponential backoff:**
- Issue: `WaitForDeploymentsReady` and `WaitForStatefulSetsReady` in `internal/kube/client.go` poll on a fixed 2-second interval with no backoff. High-frequency polling against a Kind API server can slow cluster startup.
- Files: `internal/kube/client.go:264-330`, `internal/kube/client.go:334-406`
- Impact: Unnecessary API load during cluster provisioning; fixed 2 second sleep is also brittle for slow CI runners.
- Fix approach: Use `wait.PollUntilContextTimeout` from `k8s.io/apimachinery/pkg/util/wait` with backoff, or watch resources instead of polling.

**All API calls use `context.Background()` instead of propagating the command context:**
- Issue: Every function in `internal/api/client.go` creates its own `context.Background()`. When the user presses Ctrl-C, the Cobra command context is cancelled, but the in-flight HTTP requests to the backend continue until the 30-second HTTP client timeout.
- Files: `internal/api/client.go:31`, `internal/api/client.go:55`, `internal/api/client.go:88`, `internal/api/client.go:121`, `internal/api/client.go:149`, `internal/api/client.go:195`, `internal/api/client.go:260`, `internal/api/client.go:287`
- Impact: CLI appears frozen after Ctrl-C until the HTTP timeout fires. Can also cause partial writes (e.g., a challenge submit that completes despite the user cancelling).
- Fix approach: Accept a `ctx context.Context` parameter in all API functions and pass it through.

---

## Security Considerations

**Shell injection surface in connectivity validation:**
- Risk: `executeConnectivity` in `internal/validation/executor.go` builds a `curl` shell command using `sh -c` executed inside the target pod. The URL value comes from `challenge.yaml` fetched from GitHub. The `escapeShellArg` function escapes single quotes, but the URL is still constructed via `fmt.Sprintf` into a shell string. If there is a bug in `escapeShellArg` or if the challenge YAML is compromised upstream, arbitrary shell commands could run inside the pod.
- Files: `internal/validation/executor.go:432-438`, `internal/validation/executor.go:440-522`
- Current mitigation: `escapeShellArg` handles the most common injection vector (embedded single quotes). The challenge YAML source is restricted to a single trusted GitHub repository via URL prefix check.
- Recommendations: Prefer using `exec.Command` with individual args via `remotecommand` rather than `sh -c` to avoid the shell entirely. Pass the URL as a positional argument to curl, not embedded in a shell string.

**JWT token used as raw API key without expiry enforcement:**
- Risk: The API key stored via `keystore` is a JWT token, but expiry is only shown informatively in the login flow (`cmd/login.go:47-51`) — it is never enforced before making API calls. An expired token is silently sent to the server and only fails at the backend.
- Files: `cmd/login.go:43-53`, `internal/keystore/keystore.go:76-106`
- Current mitigation: Backend rejects expired tokens; user gets an error message suggesting re-login.
- Recommendations: Check `exp` claim before making API calls and prompt re-login proactively, rather than propagating a request known to fail.

**`FetchManifest` has no URL validation:**
- Risk: `kube.FetchManifest` accepts an arbitrary URL and performs an HTTP GET without any restriction. It is currently called only from `infrastructure.go` with hardcoded GitHub/Kyverno URLs, but the function is exported and could be called with attacker-controlled URLs.
- Files: `internal/kube/manifest.go:21-34`
- Current mitigation: All current call sites pass trusted hardcoded URLs.
- Recommendations: Accept a URL allowlist parameter, or make the function unexported and restrict callers.

---

## Performance Bottlenecks

**Serial deployment wait for multi-component challenges:**
- Problem: `WaitForDeploymentsReady` and `WaitForStatefulSetsReady` in `deployer.WaitForChallengeReady` wait for each resource sequentially. If a challenge has two deployments, it waits for the first to be fully ready before starting to poll the second.
- Files: `internal/deployer/challenge.go:114-148`, `internal/kube/client.go:258-331`
- Cause: Sequential polling loop, one deployment at a time.
- Improvement path: Parallel readiness checks using goroutines and `sync.WaitGroup`, or use `kubectl wait --for=condition=ready` semantics via the watch API.

**REST mapper built from full API discovery on every deploy:**
- Problem: `restmapper.GetAPIGroupResources` performs a full cluster API discovery (many HTTP calls) each time `DeployChallenge` or `DeployLocalChallenge` is called. Discovery is repeated even when deploying multiple challenges in sequence.
- Files: `internal/deployer/challenge.go:41-46`, `internal/deployer/local.go:22-27`, `internal/deployer/infrastructure.go:54-58`
- Cause: No caching of the REST mapper between calls.
- Improvement path: Cache the mapper at the command level or use a lazy/cached REST mapper.

---

## Fragile Areas

**`getGVRForKind` hardcoded kind→GVR map:**
- Files: `internal/validation/executor.go:587-607`
- Why fragile: Only handles `Deployment`, `StatefulSet`, `DaemonSet`, `ReplicaSet`, `Job`, `Pod`, `Service`. Any challenge using a CronJob, ConfigMap, Ingress, PersistentVolumeClaim, or any custom resource will return an "unsupported resource kind" error from the `status` validation type.
- Safe modification: Add new cases to the switch statement. Do not rely on dynamic discovery because the static map is intentional for security (avoids accepting arbitrary kinds).
- Test coverage: Not covered by unit tests for the unsupported-kind path in production validation flows.

**Challenge namespace equals slug with no isolation:**
- Files: `cmd/start.go:95`, `cmd/submit.go:95`
- Why fragile: The namespace for a challenge is `challengeSlug` verbatim. If a user simultaneously runs two challenges with slugs that differ only in ways that Kubernetes namespace naming doesn't distinguish, or if a slug collides with a system namespace name, the commands will operate on the wrong namespace.
- Safe modification: Always validate slug format before using it as a namespace name. `validateChallengeSlug` is available but is not called in `start.go` or `submit.go`.

**`slug` validation inconsistently applied across commands:**
- Files: `cmd/start.go`, `cmd/submit.go`, `cmd/reset.go`, `cmd/clean.go`
- Why fragile: `validateChallengeSlug` is defined in `cmd/common.go` and called in dev commands (`dev_logs.go`, `dev_validate.go`, `dev_clean.go`, `dev_apply.go`, `dev_status.go`, `dev_get.go`) but is NOT called in the production user-facing commands `start`, `submit`, `reset`, or `clean`. Malformed slugs passed to these commands are sent directly to the Kubernetes API as namespace names.
- Safe modification: Add `validateChallengeSlug(slug)` at the top of each command's `RunE` before any API or cluster call.

**`getRestConfig()` always wraps transport when logger is non-nil:**
- Files: `internal/kube/client.go:90-95`
- Why fragile: The `LoggingRoundTripper` wraps the transport on every client creation, even when the logger level is not DEBUG. This adds overhead on every Kubernetes API call regardless of debug mode.
- Safe modification: Gate the transport wrapping on whether the logger level is actually DEBUG before wrapping.

---

## Test Coverage Gaps

**Production user commands (`start`, `submit`, `reset`, `clean`) have no unit tests:**
- What's not tested: The `RunE` implementations in `cmd/start.go`, `cmd/submit.go`, `cmd/reset.go`, `cmd/clean.go` — the primary user-facing flows.
- Files: `cmd/start.go`, `cmd/submit.go`, `cmd/reset.go`, `cmd/clean.go`
- Risk: Regressions in the main user workflow (slug validation, progress state machine, API call sequence) go undetected until integration or manual testing.
- Priority: High

**`getGVRForKind` unsupported kinds not tested:**
- What's not tested: That validation with a kind not in the static map returns a clear error rather than a panic.
- Files: `internal/validation/executor.go:587-607`
- Risk: Adding a new validation type that uses an unsupported kind silently produces a misleading error at runtime.
- Priority: Medium

**`FindLocalChallengeFile` developer path fallback not tested:**
- What's not tested: The hardcoded `~/Workspace/kubeasy/challenges/` path is discovered and used in production builds.
- Files: `internal/validation/loader.go:52`
- Risk: Developer-environment files override production GitHub fetches on machines with that directory.
- Priority: Medium

---

## Scaling Limits

**Log validation buffers entire pod log into memory:**
- Current capacity: Works for typical application logs.
- Limit: For pods with large log volumes (>100MB), `io.ReadAll` in `executeLog` will allocate the entire log into a single `[]byte` per pod before scanning for strings. If multiple pods are matched by a label selector, memory usage multiplies.
- Files: `internal/validation/executor.go:288-296`
- Scaling path: Use streaming log scanning with `bufio.Scanner` to avoid holding the full log in memory.

---

*Concerns audit: 2026-03-09*
