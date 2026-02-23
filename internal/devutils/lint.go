package devutils

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/kubeasy-dev/kubeasy-cli/internal/validation"
	"gopkg.in/yaml.v3"
)

// LintSeverity represents the severity of a lint issue.
type LintSeverity string

const (
	SeverityError   LintSeverity = "error"
	SeverityWarning LintSeverity = "warning"
)

// LintIssue represents a single lint finding.
type LintIssue struct {
	Field    string
	Severity LintSeverity
	Message  string
}

var (
	ValidDifficulties = []string{"easy", "medium", "hard"}
	ValidTypes        = []string{"fix", "build", "migrate"}
)

// LintChallengeFile validates a challenge.yaml file structure without requiring a cluster.
func LintChallengeFile(path string) ([]LintIssue, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	var issues []LintIssue

	// Check required string fields
	requiredStrings := []string{"title", "description", "theme", "difficulty", "type", "initialSituation", "objective"}
	for _, field := range requiredStrings {
		val, ok := raw[field]
		if !ok || val == nil {
			issues = append(issues, LintIssue{Field: field, Severity: SeverityError, Message: fmt.Sprintf("required field '%s' is missing", field)})
		} else if s, ok := val.(string); ok && s == "" {
			issues = append(issues, LintIssue{Field: field, Severity: SeverityError, Message: fmt.Sprintf("required field '%s' is empty", field)})
		}
	}

	// Check estimatedTime
	if et, ok := raw["estimatedTime"]; !ok || et == nil {
		issues = append(issues, LintIssue{Field: "estimatedTime", Severity: SeverityError, Message: "required field 'estimatedTime' is missing"})
	} else {
		switch v := et.(type) {
		case int:
			if v <= 0 {
				issues = append(issues, LintIssue{Field: "estimatedTime", Severity: SeverityError, Message: "estimatedTime must be greater than 0"})
			}
		case float64:
			if v <= 0 {
				issues = append(issues, LintIssue{Field: "estimatedTime", Severity: SeverityError, Message: "estimatedTime must be greater than 0"})
			}
		default:
			issues = append(issues, LintIssue{Field: "estimatedTime", Severity: SeverityError, Message: "estimatedTime must be a number"})
		}
	}

	// Check difficulty value
	if diff, ok := raw["difficulty"].(string); ok && diff != "" {
		if !slices.Contains(ValidDifficulties, diff) {
			issues = append(issues, LintIssue{Field: "difficulty", Severity: SeverityError, Message: fmt.Sprintf("invalid difficulty '%s' (valid: %v)", diff, ValidDifficulties)})
		}
	}

	// Check type value
	if t, ok := raw["type"].(string); ok && t != "" {
		if !slices.Contains(ValidTypes, t) {
			issues = append(issues, LintIssue{Field: "type", Severity: SeverityError, Message: fmt.Sprintf("invalid type '%s' (valid: %v)", t, ValidTypes)})
		}
	}

	// Validate objectives structure via validation.Parse
	_, parseErr := validation.Parse(data)
	if parseErr != nil {
		issues = append(issues, LintIssue{Field: "objectives", Severity: SeverityError, Message: fmt.Sprintf("objectives parse error: %v", parseErr)})
	}

	// Check objective keys unique and orders sequential
	if objectives, ok := raw["objectives"].([]interface{}); ok && len(objectives) > 0 {
		keys := make(map[string]bool)
		orders := make([]int, 0, len(objectives))
		for i, obj := range objectives {
			if m, ok := obj.(map[string]interface{}); ok {
				if key, ok := m["key"].(string); ok {
					if keys[key] {
						issues = append(issues, LintIssue{Field: "objectives", Severity: SeverityError, Message: fmt.Sprintf("duplicate objective key '%s'", key)})
					}
					keys[key] = true
				} else {
					issues = append(issues, LintIssue{Field: "objectives", Severity: SeverityError, Message: fmt.Sprintf("objective %d missing 'key'", i)})
				}
				if order, ok := m["order"]; ok {
					switch v := order.(type) {
					case int:
						orders = append(orders, v)
					case float64:
						orders = append(orders, int(v))
					}
				}
			}
		}
		// Check orders are sequential starting from 1
		if len(orders) == len(objectives) {
			slices.Sort(orders)
			for i, o := range orders {
				if o != i+1 {
					issues = append(issues, LintIssue{Field: "objectives", Severity: SeverityWarning, Message: fmt.Sprintf("objective orders are not sequential (expected %d, got %d)", i+1, o)})
					break
				}
			}
		}
	}

	// Check manifests/ directory exists and is non-empty
	dir := filepath.Dir(path)
	manifestsDir := filepath.Join(dir, "manifests")
	entries, err := os.ReadDir(manifestsDir)
	if err != nil {
		issues = append(issues, LintIssue{Field: "manifests/", Severity: SeverityWarning, Message: "manifests/ directory not found"})
	} else {
		hasManifests := false
		for _, e := range entries {
			if !e.IsDir() && e.Name() != ".gitkeep" {
				hasManifests = true
				break
			}
		}
		if !hasManifests {
			issues = append(issues, LintIssue{Field: "manifests/", Severity: SeverityWarning, Message: "manifests/ directory is empty (no YAML files)"})
		}
	}

	return issues, nil
}
