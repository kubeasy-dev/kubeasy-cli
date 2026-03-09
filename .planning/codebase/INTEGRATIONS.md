# External Integrations

**Analysis Date:** 2026-03-09

## APIs & External Services

**Kubeasy Backend API (primary):**
- Service: `kubeasy.dev` REST API (Next.js backend with OpenAPI spec)
- Purpose: Challenge metadata, user progress, submission results, authentication verification
- SDK/Client: Generated via `oapi-codegen` from `openapi.json` → `internal/apigen/client.gen.go`
- Wrapper: `internal/api/client.go`, `internal/api/auth.go`
- Base URL: `https://kubeasy.dev` (injected at build time via ldflags into `constants.WebsiteURL`)
- Auth: Bearer token from keystore, injected via `BearerAuthEditorFn` in `internal/api/auth.go`
- Endpoints used:
  - `GET /api/cli/user` - Fetch user profile
  - `POST /api/cli/user` - Login/register CLI session with OS/version metadata
  - `GET /api/cli/challenge/[slug]` - Fetch challenge details
  - `GET /api/cli/challenge/[slug]/status` - Check user's progress
  - `POST /api/cli/challenge/[slug]/start` - Record challenge start
  - `POST /api/cli/challenge/[slug]/submit` - Submit validation results
  - `POST /api/cli/challenge/[slug]/reset` - Reset progress
  - `POST /api/cli/track/setup` - Track setup events
  - `GET /api/cli/types`, `/api/cli/themes`, `/api/cli/difficulties` - Public catalog metadata

**GitHub Raw Content (challenges repo):**
- Service: `raw.githubusercontent.com/kubeasy-dev/challenges/main`
- Purpose: Fetch `challenge.yaml` for validation specs when no local file is found
- Auth: None (public repo)
- Used in: `internal/validation/loader.go` → `loadFromURL()`
- Security: URL is validated against `ChallengesRepoBaseURL` constant before fetch

**GitHub Container Registry (GHCR):**
- Service: `ghcr.io/kubeasy-dev/challenges`
- Purpose: OCI artifact registry for challenge manifests (Kubernetes YAML + Kyverno policies)
- Auth: None (public registry, anonymous pull)
- Used in: `internal/deployer/challenge.go` → `pullOCIArtifact()` via `oras-go`
- Artifact format: `ghcr.io/kubeasy-dev/challenges/<slug>:latest`

**Kyverno GitHub Releases:**
- Service: `github.com/kyverno/kyverno/releases/download/<version>/install.yaml`
- Purpose: Download Kyverno admission controller install manifest during `kubeasy setup`
- Auth: None (public)
- Version: Managed by Renovate in `internal/deployer/const.go`

**Rancher GitHub Raw Content (local-path-provisioner):**
- Service: `raw.githubusercontent.com/rancher/local-path-provisioner/<version>/deploy/local-path-storage.yaml`
- Purpose: Download local-path-provisioner storage class manifest during `kubeasy setup`
- Auth: None (public)
- Version: Managed by Renovate in `internal/deployer/const.go`

## Data Storage

**Databases:**
- None local - all persistence is in the Kubeasy backend (kubeasy.dev)

**File Storage:**
- Kubeconfig: Standard kubeconfig (`~/.kube/config`) - read and modified by `internal/kube/config.go`
- Credentials: `~/.config/kubeasy-cli/credentials` (Linux/macOS) or `%APPDATA%/kubeasy-cli/credentials` (Windows) - JSON file with API key, managed by `internal/keystore/`
- Logs: `/tmp/kubeasy-cli.log` (when `--debug` flag is active)
- OCI artifacts: Temporary directory (`os.MkdirTemp`) - cleaned up after deployment

**Caching:**
- None

## Authentication & Identity

**Auth Provider:**
- Custom JWT-based authentication via `kubeasy.dev` backend (backed by Supabase on the server side)
- Implementation:
  - User runs `kubeasy login`, enters API key (JWT token from kubeasy.dev website)
  - Token stored in system keyring (preferred) or file fallback via `internal/keystore/`
  - Storage priority: `KUBEASY_API_KEY` env var > system keyring > `~/.config/kubeasy-cli/credentials`
  - All API requests inject Bearer token via `BearerAuthEditorFn` in `internal/api/auth.go`
  - Token validation (JWT parsing) in `internal/api/` uses `github.com/golang-jwt/jwt/v5`

**System Keyring:**
- Linux: GNOME Keyring or KDE Wallet via `github.com/zalando/go-keyring`
- macOS: macOS Keychain
- Windows: Windows Credential Manager
- Fallback for headless environments: file-based storage with owner-only permissions (0600/ACL)

## Monitoring & Observability

**Error Tracking:**
- None (no Sentry, Datadog, etc.)

**Logs:**
- Custom logger in `internal/logger/logger.go`
- File output at `/tmp/kubeasy-cli.log` when `--debug` flag is passed to root command
- Levels: DEBUG, INFO, WARN, ERROR
- Max log lines: 1000 (rotated, controlled by `constants.MaxLogLines`)

**Telemetry:**
- Minimal: `TrackSetup()` sends CLI version, OS, and arch to `/api/cli/track/setup` on `kubeasy setup`
- Login call sends same metadata to `/api/cli/user` (POST) for usage tracking

## CI/CD & Deployment

**Hosting:**
- Binary CDN: Cloudflare R2 (`https://download.kubeasy.dev`, bucket `kubeasy-cli-binaries`, endpoint `57e2edb42742bf00d9a2526736f3ea36.r2.cloudflarestorage.com`)
- Package registries: NPM (`@kubeasy-dev/kubeasy-cli`), Homebrew tap (`kubeasy-dev/homebrew-tap`), Scoop bucket (`kubeasy-dev/scoop-bucket`)
- Releases: GitHub Releases (binaries + checksums)

**CI Pipeline:**
- GitHub Actions (`.github/workflows/`)
- `test.yml` - Unit, integration, and e2e tests on push to main and PRs; uploads coverage to Codecov
- `lint.yml` - golangci-lint on push/PR
- `releasing.yml` - Tag-triggered: pre-release checks → GoReleaser build → NPM publish → auto-merge website schema PR
- `release-dispatch.yml` - Manual release dispatch workflow
- `generate-zod-schema.yaml` - Generates Zod schema from OpenAPI spec and opens PR to website repo
- `update-openapi-client.yaml` - Updates generated API client when `openapi.json` changes
- `claude-code-review.yml`, `claude.yml` - Claude AI code review integrations
- `issue-triage.yml` - Automated issue triage
- `scheduled-maintenance.yml` - Scheduled maintenance tasks

**Coverage:**
- Codecov (`codecov.yaml`) - Uploads unit and integration coverage from `coverage-unit.out` / `coverage-integration.out`

## Environment Configuration

**Required for release (CI secrets):**
- `GITHUB_TOKEN` - GoReleaser GitHub release creation
- `R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY` - Cloudflare R2 binary upload
- `NPM_TOKEN` - NPM publishing
- `APP_ID` / `APP_PRIVATE_KEY` - GitHub App token for cross-repo operations (homebrew-tap, scoop-bucket, website)
- `TAP_GITHUB_TOKEN` - Generated by GitHub App for Homebrew/Scoop manifest updates

**Required at runtime (user machine):**
- `KUBEASY_API_KEY` (optional) - API key override, takes priority over keyring/file storage
- Docker (for Kind cluster creation)
- `~/.kube/config` - Standard kubeconfig, must be present or creatable

**Secrets location:**
- Runtime credentials: system keyring or `~/.config/kubeasy-cli/credentials`
- CI secrets: GitHub repository secrets

## Webhooks & Callbacks

**Incoming:**
- None

**Outgoing:**
- Release pipeline auto-merges a PR in `kubeasy-dev/website` repo (branch `chore/update-schema-from-cli`) after NPM publish, triggered via GitHub App token in `releasing.yml`

---

*Integration audit: 2026-03-09*
