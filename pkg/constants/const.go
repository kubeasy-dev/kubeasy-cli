package constants

import "strings"

// WebsiteURL is the base URL for the Kubeasy website (used for CLI API routes)
var WebsiteURL = "http://localhost:3000"

var RestAPIUrl = WebsiteURL + "/api/cli"

var KeyringServiceName = "kubeasy-cli"

var GithubRootURL = "https://github.com/kubeasy-dev"

var ExercisesRepoURL = GithubRootURL + "/challenges"

var ExercicesRepoBranch = "main"

var Version = "dev"

var KubeasyClusterContext = "kind-kubeasy"

// KindNodeImage is the container image used for Kind cluster nodes.
// This is the single source of truth for the Kubernetes version used by Kind.
// The version should match the k8s.io/* library versions in go.mod (v0.X.Y -> 1.X.Y).
//
// IMPORTANT: This constant is managed by Renovate. The comment format below
// is required for automatic updates. Do not modify the format.
// renovate: datasource=docker depName=kindest/node
var KindNodeImage = "kindest/node:v1.35.0"

// GetKubernetesVersion extracts the Kubernetes version from KindNodeImage.
// Returns the version without the "v" prefix (e.g., "1.35.0").
func GetKubernetesVersion() string {
	// KindNodeImage format: "kindest/node:v1.35.0"
	parts := strings.SplitN(KindNodeImage, ":", 2)
	if len(parts) != 2 {
		return "unknown"
	}
	return strings.TrimPrefix(parts[1], "v")
}

// LogFilePath defines the default path for the log file when debug is enabled
var LogFilePath = "kubeasy-cli.log"

var MaxLogLines = 1000 // Maximum number of lines to keep in the log file
