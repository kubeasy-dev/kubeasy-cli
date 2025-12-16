[![codecov](https://codecov.io/gh/kubeasy-dev/kubeasy-cli/graph/badge.svg?token=7M8PHNZAM6)](https://codecov.io/gh/kubeasy-dev/kubeasy-cli)

# kubeasy-cli

A command-line tool to learn Kubernetes through practical challenges. Create local Kind clusters, deploy challenges via ArgoCD, and validate solutions using a Kubernetes operator.

## Features

- Cross-platform support (Linux, macOS, Windows)
- Local Kind cluster management with ArgoCD
- Challenge deployment and validation
- Progress tracking with backend integration
- 6 specialized validation types for comprehensive testing

## Installation

```bash
# Via npm
npm install -g @kubeasy-dev/kubeasy-cli
```

## Usage

```bash
# Setup local environment (creates Kind cluster with ArgoCD)
kubeasy setup

# Login with API key
kubeasy login

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

## Documentation

See [online docs](https://docs.kubeasy.dev/developer/contributing) for contribution guidelines, setup instructions, and development workflow.

See [CLAUDE.md](./CLAUDE.md) for detailed architecture and development guidance.

## License

See LICENSE file for details.
