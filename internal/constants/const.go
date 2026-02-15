package constants

import "strings"

// WebsiteURL is the base URL for the Kubeasy website (used for CLI API routes)
var WebsiteURL = "http://localhost:3000"

var KeyringServiceName = "kubeasy-cli"

var GithubRootURL = "https://github.com/kubeasy-dev"

var ExercisesRepoURL = GithubRootURL + "/challenges"

var ExercicesRepoBranch = "challenge/refactoring-2024"

var Version = "dev"

var KubeasyClusterContext = "kind-kubeasy"

// UnknownVersion is returned when a version cannot be parsed or determined.
const UnknownVersion = "unknown"

// KindNodeImage is the container image used for Kind cluster nodes.
// This is the single source of truth for the Kubernetes version used by Kind.
// The version should match the k8s.io/* library versions in go.mod (v0.X.Y -> 1.X.Y).
//
// Note: This is a var (not const) to allow Renovate to update it automatically.
// IMPORTANT: The comment format below is required for Renovate. Do not modify.
// renovate: datasource=docker depName=kindest/node
var KindNodeImage = "kindest/node:v1.35.0"

// GetKubernetesVersion extracts the Kubernetes version from KindNodeImage.
// Returns the version without the "v" prefix (e.g., "1.35.0").
// Handles prerelease versions (e.g., "1.35.0-alpha.1" returns "1.35.0-alpha.1").
func GetKubernetesVersion() string {
	// KindNodeImage format: "kindest/node:v1.35.0" or "kindest/node:v1.35.0-alpha.1"
	parts := strings.SplitN(KindNodeImage, ":", 2)
	if len(parts) != 2 {
		return UnknownVersion
	}
	return strings.TrimPrefix(parts[1], "v")
}

// GetMajorMinorVersion extracts just the major.minor part from a version string.
// Handles various formats: "1.35.0", "1.35.0+k3s1", "1.35.0-eks", "v1.35.0".
// Returns UnknownVersion if the version cannot be parsed.
func GetMajorMinorVersion(version string) string {
	// Remove leading "v" if present
	v := strings.TrimPrefix(version, "v")

	// Strip build metadata and prerelease info by finding the first separator
	// Handles formats like "1.35.0+k3s1", "1.35.0-eks", or "1.35.0-rc.1+build123"
	if idx := strings.IndexAny(v, "+-"); idx >= 0 {
		v = v[:idx]
	}

	// Extract major.minor from "major.minor.patch"
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return UnknownVersion
	}

	return parts[0] + "." + parts[1]
}

// VersionsCompatible checks if two Kubernetes versions are compatible.
// Two versions are considered compatible if they share the same major.minor version.
// This handles build metadata (+k3s1, -eks) and patch version differences.
func VersionsCompatible(version1, version2 string) bool {
	mm1 := GetMajorMinorVersion(version1)
	mm2 := GetMajorMinorVersion(version2)

	if mm1 == UnknownVersion || mm2 == UnknownVersion {
		return false
	}

	return mm1 == mm2
}

// LogFilePath defines the default path for the log file when debug is enabled
var LogFilePath = "kubeasy-cli.log"

var MaxLogLines = 1000 // Maximum number of lines to keep in the log file
