package constants

import (
	"os"
	"path/filepath"
	"strings"
)

// WebsiteURL is the base URL for the Kubeasy website (used for CLI API routes)
var WebsiteURL = "https://kubeasy.dev"

func init() {
	if v := os.Getenv("KUBEASY_API_URL"); v != "" {
		WebsiteURL = v
	} else if v := os.Getenv("API_URL"); v != "" {
		WebsiteURL = v
	}
}

var KeyringServiceName = "kubeasy-cli"

var GithubRootURL = "https://github.com/kubeasy-dev"

var KubeasyClusterContext = "kind-kubeasy"
var KubeasyClusterName = "kubeasy"

var DownloadBaseURL = "https://download.kubeasy.dev"

// Version is the current version of the CLI.
// It is set at build time via LDFLAGS.
var Version = "dev"

// LogFilePath is the path where CLI logs are stored.
// It is set at build time via LDFLAGS.
var LogFilePath = "/tmp/kubeasy-cli.log"

// MaxLogLines is the maximum number of lines allowed in the log file.
const MaxLogLines = 5000

// GetKubeasyConfigDir returns the path to the CLI configuration directory (~/.config/kubeasy-cli)
func GetKubeasyConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/kubeasy-cli" // Fallback
	}
	return filepath.Join(home, ".kubeasy")
}

// GetCredentialsFilePath returns the path to the CLI credentials file
func GetCredentialsFilePath() (string, error) {
	configDir := GetKubeasyConfigDir()
	return filepath.Join(configDir, "credentials"), nil
}

// UnknownVersion is returned when a version cannot be determined.
const UnknownVersion = "unknown"

// KindNodeImage is the default Kind node image to use.
const KindNodeImage = "kindest/node:v1.35.0"

// GetKubernetesVersion extracts the Kubernetes version from KindNodeImage.
func GetKubernetesVersion() string {
	parts := strings.Split(KindNodeImage, ":v")
	if len(parts) > 1 {
		return parts[1]
	}
	return UnknownVersion
}

// GetMajorMinorVersion returns the major.minor part of a version string.
func GetMajorMinorVersion(v string) string {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	if len(parts) >= 2 {
		// handle suffixes like 1.35.0-alpha.1 or 1.35.0+k3s1
		minor := parts[1]
		for i, char := range minor {
			if char < '0' || char > '9' {
				minor = minor[:i]
				break
			}
		}
		if minor == "" {
			return UnknownVersion
		}
		return parts[0] + "." + minor
	}
	return UnknownVersion
}

// VersionsCompatible compares two semver-like strings and returns true if they are compatible.
func VersionsCompatible(current, required string) bool {
	if required == "" {
		return current != ""
	}
	if current == "dev" {
		return true
	}
	v1 := GetMajorMinorVersion(current)
	v2 := GetMajorMinorVersion(required)
	if v1 == UnknownVersion || v2 == UnknownVersion {
		return false
	}
	return v1 == v2
}

// GetKindConfigPath returns the path to the Kind configuration file.
func GetKindConfigPath() string {
	return filepath.Join(GetKubeasyConfigDir(), "kind-config.yaml")
}

// GetCloudProviderKindBinPath returns the path to the cloud-provider-kind binary.
func GetCloudProviderKindBinPath() string {
	return filepath.Join(GetKubeasyConfigDir(), "bin", "cloud-provider-kind")
}

const (
	// KubeasyCASecretNamespace is the namespace where the Kubeasy CA Secret is stored.
	KubeasyCASecretNamespace = "cert-manager"
	// KubeasyCASecretName is the name of the Kubeasy CA Secret.
	// #nosec G101 (false positive: this is a resource name, not a secret value)
	KubeasyCASecretName = "kubeasy-ca"
	// KubeasyCASecretCertKey is the key holding the PEM-encoded CA certificate.
	KubeasyCASecretCertKey = "tls.crt"
	// KubeasyCAPrivateKeyField is the Secret data key holding the PEM-encoded CA private key.
	KubeasyCAPrivateKeyField = "tls.key"
)
