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
npm install -g kubeasy-cli

# Or download binary from releases
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

## Validation System

Challenges are validated using 6 specialized Custom Resource Definitions (CRDs):

- **LogValidation** - Checks container logs for expected strings
- **StatusValidation** - Validates resource status conditions (Pod Ready, Deployment Available, etc.)
- **EventValidation** - Detects forbidden Kubernetes events (BackOff, Evicted, etc.)
- **MetricsValidation** - Verifies pod/deployment metrics (restart count, replicas, etc.)
- **RBACValidation** - Tests ServiceAccount permissions using SubjectAccessReview
- **ConnectivityValidation** - Validates network connectivity between pods

The `submit` command automatically discovers all validation CRDs, evaluates their status, and sends structured results to the backend for progress tracking.

## Development

See [CLAUDE.md](./CLAUDE.md) for detailed architecture and development guidance.

## Contributing

See [online docs](https://docs.kubeasy.dev/developer/contributing) for contribution guidelines, setup instructions, and development workflow.

## License

See LICENSE file for details.
