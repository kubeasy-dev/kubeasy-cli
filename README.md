[![codecov](https://codecov.io/gh/kubeasy-dev/kubeasy-cli/graph/badge.svg?token=7M8PHNZAM6)](https://codecov.io/gh/kubeasy-dev/kubeasy-cli)

# kubeasy-cli

A command-line tool to learn Kubernetes through practical challenges. Create local Kind clusters, deploy challenges via OCI artifacts, and validate solutions using a CLI-based validation system.

## Features

- Cross-platform support (Linux, macOS, Windows)
- Local Kind cluster management with Kyverno
- Challenge deployment and validation
- Progress tracking with backend integration
- 5 specialized validation types for comprehensive testing
- Dev mode for challenge creators (scaffold, deploy, validate, lint locally)

## Installation

### Homebrew (macOS)

```bash
brew install kubeasy-dev/homebrew-tap/kubeasy
```

### Scoop (Windows)

```powershell
scoop bucket add kubeasy https://github.com/kubeasy-dev/scoop-bucket
scoop install kubeasy
```

### NPM

```bash
npm install -g @kubeasy-dev/kubeasy-cli
```

### Shell script (Linux/macOS)

```bash
curl -fsSL https://download.kubeasy.dev/install.sh | sh
```

### GitHub Releases

Download the latest release from the [releases page](https://github.com/kubeasy-dev/kubeasy-cli/releases/latest).

## Documentation

For usage instructions, CLI commands, and contribution guidelines, see the [online documentation](https://docs.kubeasy.dev).

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
