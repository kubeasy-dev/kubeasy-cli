package validation

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	// ChallengesRepoBaseURL is the base URL for the challenges repository
	ChallengesRepoBaseURL = "https://raw.githubusercontent.com/kubeasy-dev/challenges/main"
)

// LoadFromFile loads validations from a local file
func LoadFromFile(path string) (*ValidationConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return Parse(data)
}

// FindLocalChallengeFile looks for challenge.yaml in common locations
func FindLocalChallengeFile(slug string) string {
	// Check common development paths
	paths := []string{
		filepath.Join(".", slug, "challenge.yaml"),
		filepath.Join("..", "challenges", slug, "challenge.yaml"),
		filepath.Join(os.Getenv("HOME"), "Workspace", "kubeasy", "challenges", slug, "challenge.yaml"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// LoadFromURL loads validations from a remote URL
// This function only fetches from the known challenges repository base URL
func LoadFromURL(url string) (*ValidationConfig, error) {
	resp, err := http.Get(url) //nolint:gosec // URL is constructed from trusted ChallengesRepoBaseURL
	if err != nil {
		return nil, fmt.Errorf("failed to fetch validations: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch validations: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return Parse(data)
}

// LoadForChallenge loads validations for a specific challenge
// It tries local file first (for development), then falls back to GitHub
func LoadForChallenge(slug string) (*ValidationConfig, error) {
	// Try local file first (for development)
	if localPath := FindLocalChallengeFile(slug); localPath != "" {
		return LoadFromFile(localPath)
	}

	// Fall back to GitHub
	url := fmt.Sprintf("%s/%s/challenge.yaml", ChallengesRepoBaseURL, slug)
	return LoadFromURL(url)
}

// Parse parses validation config from YAML data
func Parse(data []byte) (*ValidationConfig, error) {
	var config ValidationConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse validations: %w", err)
	}

	// Parse specs into typed structs
	for i := range config.Validations {
		if err := parseSpec(&config.Validations[i]); err != nil {
			return nil, fmt.Errorf("failed to parse spec for %s: %w", config.Validations[i].Key, err)
		}
	}

	return &config, nil
}

// parseSpec converts RawSpec to the appropriate typed spec based on Type
func parseSpec(v *Validation) error {
	if v.RawSpec == nil {
		return fmt.Errorf("spec is required")
	}

	// Convert RawSpec back to YAML then to typed struct
	specYAML, err := yaml.Marshal(v.RawSpec)
	if err != nil {
		return err
	}

	switch v.Type {
	case TypeStatus:
		var spec StatusSpec
		if err := yaml.Unmarshal(specYAML, &spec); err != nil {
			return err
		}
		v.Spec = spec

	case TypeLog:
		var spec LogSpec
		if err := yaml.Unmarshal(specYAML, &spec); err != nil {
			return err
		}
		// Default sinceSeconds to 300 (5 minutes)
		if spec.SinceSeconds == 0 {
			spec.SinceSeconds = 300
		}
		v.Spec = spec

	case TypeEvent:
		var spec EventSpec
		if err := yaml.Unmarshal(specYAML, &spec); err != nil {
			return err
		}
		// Default sinceSeconds to 300 (5 minutes)
		if spec.SinceSeconds == 0 {
			spec.SinceSeconds = 300
		}
		v.Spec = spec

	case TypeMetrics:
		var spec MetricsSpec
		if err := yaml.Unmarshal(specYAML, &spec); err != nil {
			return err
		}
		v.Spec = spec

	case TypeConnectivity:
		var spec ConnectivitySpec
		if err := yaml.Unmarshal(specYAML, &spec); err != nil {
			return err
		}
		// Default timeout
		for i := range spec.Targets {
			if spec.Targets[i].TimeoutSeconds == 0 {
				spec.Targets[i].TimeoutSeconds = 5
			}
		}
		v.Spec = spec

	default:
		return fmt.Errorf("unknown validation type: %s", v.Type)
	}

	return nil
}
