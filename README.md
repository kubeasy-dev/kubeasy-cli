[![codecov](https://codecov.io/gh/kubeasy-dev/kubeasy-cli/graph/badge.svg?token=7M8PHNZAM6)](https://codecov.io/gh/kubeasy-dev/kubeasy-cli)

# kubeasy-cli

A command-line tool to learn Kubernetes through practical challenges. Create local Kind clusters, deploy challenges via ArgoCD, and validate solutions using a Kubernetes operator.

## Features

- Cross-platform support (Linux, macOS, Windows)
- Local Kind cluster management with ArgoCD
- Challenge deployment and validation
- Progress tracking with backend integration
- 6 specialized validation types for comprehensive testing
- Dev mode for challenge creators (scaffold, deploy, validate, lint locally)

## Installation

```bash
# Via npm
npm install -g @kubeasy-dev/kubeasy-cli
```

## Usage

```bash
# Login with API key (required before setup)
kubeasy login

# Setup local environment (creates Kind cluster with ArgoCD)
kubeasy setup

# Get challenge information
kubeasy challenge get <challenge-slug>

# Start a challenge
kubeasy challenge start <challenge-slug>

# Submit your solution
kubeasy challenge submit <challenge-slug>

# Reset challenge (clears resources and backend progress)
kubeasy challenge reset <challenge-slug>

# Clean challenge (removes resources only, keeps progress)
kubeasy challenge clean <challenge-slug>
```

## Dev Mode (Challenge Creators)

```bash
# Scaffold a new challenge directory
kubeasy dev create

# Display local challenge metadata (no cluster needed)
kubeasy dev get

# Validate challenge.yaml structure (no cluster needed)
kubeasy dev lint

# Deploy local manifests to the Kind cluster
kubeasy dev apply

# Run validations locally without submitting to API
kubeasy dev validate

# Apply manifests and run validations in one step
kubeasy dev test

# Show pods, events, and objective count for a deployed challenge
kubeasy dev status

# Stream logs from challenge pods
kubeasy dev logs

# Remove dev challenge resources from the cluster
kubeasy dev clean
```

## Documentation

See [online docs](https://docs.kubeasy.dev/developer/contributing) for contribution guidelines, setup instructions, and development workflow.

See [CLAUDE.md](./CLAUDE.md) for detailed architecture and development guidance.

## Telemetry

Kubeasy CLI collects minimal, anonymous usage telemetry to help improve the tool. Tracking events are sent during `login` and `setup` commands when you are authenticated.

**Data collected:**
- CLI version
- Operating system (e.g. `linux`, `darwin`)
- Architecture (e.g. `amd64`, `arm64`)

**No personal information is collected.** Telemetry is only sent when authenticated and never blocks CLI execution (fire-and-forget with a 5s timeout).

If you are not logged in, no telemetry is sent.

## License

See LICENSE file for details.
