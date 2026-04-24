package devutils

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/kubeasy-dev/kubeasy-cli/internal/validation"
	"github.com/kubeasy-dev/registry/pkg/challenges"
	"go.yaml.in/yaml/v3"
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

// LintChallengeFile validates a challenge.yaml file structure without requiring a cluster.
// Also checks that the manifests/ directory exists next to the file.
func LintChallengeFile(path string) ([]LintIssue, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	issues, err := LintChallengeData(data)
	if err != nil {
		return nil, err
	}
	// Validate manifests/ directory alongside the file using the registry's shared validator.
	challengeDir := filepath.Dir(path)
	for _, ve := range challenges.ValidateManifests(challengeDir) {
		issues = append(issues, LintIssue{Field: ve.Field, Severity: SeverityError, Message: ve.Message})
	}
	return issues, nil
}

// LintChallengeData validates challenge.yaml bytes without requiring a file on disk.
func LintChallengeData(data []byte) ([]LintIssue, error) {
	// Unmarshal into the canonical ChallengeYamlSpec struct — single source of truth.
	var spec validation.ChallengeYamlSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	var issues []LintIssue

	// Validate required fields via reflection over ChallengeYamlSpec.
	// Fields without omitempty are required; fields with omitempty are optional.
	// Slices (Objectives) are skipped here and validated separately below.
	issues = append(issues, validateRequiredFields(spec)...)

	// Check difficulty value against the canonical list.
	if spec.Difficulty != "" && !slices.Contains(validation.ChallengeDifficultyValues, spec.Difficulty) {
		issues = append(issues, LintIssue{
			Field:    "difficulty",
			Severity: SeverityError,
			Message:  fmt.Sprintf("invalid difficulty %q (valid: %v)", spec.Difficulty, validation.ChallengeDifficultyValues),
		})
	}

	// Check type value against the canonical list.
	if spec.Type != "" && !slices.Contains(validation.ChallengeTypeValues, spec.Type) {
		issues = append(issues, LintIssue{
			Field:    "type",
			Severity: SeverityError,
			Message:  fmt.Sprintf("invalid type %q (valid: %v)", spec.Type, validation.ChallengeTypeValues),
		})
	}

	// Validate objectives structure via validation.Parse (deep spec validation).
	if _, parseErr := validation.Parse(data); parseErr != nil {
		issues = append(issues, LintIssue{
			Field:    "objectives",
			Severity: SeverityError,
			Message:  fmt.Sprintf("objectives parse error: %v", parseErr),
		})
	}

	// Check objective keys are unique and orders are sequential.
	if len(spec.Objectives) > 0 {
		keys := make(map[string]bool)
		orders := make([]int, 0, len(spec.Objectives))
		for i, obj := range spec.Objectives {
			switch {
			case obj.Key == "":
				issues = append(issues, LintIssue{
					Field:    "objectives",
					Severity: SeverityError,
					Message:  fmt.Sprintf("objective %d missing 'key'", i),
				})
			case keys[obj.Key]:
				issues = append(issues, LintIssue{
					Field:    "objectives",
					Severity: SeverityError,
					Message:  fmt.Sprintf("duplicate objective key %q", obj.Key),
				})
			default:
				keys[obj.Key] = true
			}
			if obj.Order > 0 {
				orders = append(orders, obj.Order)
			}
		}
		if len(orders) == len(spec.Objectives) {
			slices.Sort(orders)
			for i, o := range orders {
				if o != i+1 {
					issues = append(issues, LintIssue{
						Field:    "objectives",
						Severity: SeverityWarning,
						Message:  fmt.Sprintf("objective orders are not sequential (expected %d, got %d)", i+1, o),
					})
					break
				}
			}
		}
	}

	return issues, nil
}

// validateRequiredFields inspects a ChallengeYamlSpec via reflection and returns
// lint errors for any required field that is missing or zero-valued.
// Required = no "omitempty" in yaml tag. Slice fields are skipped (validated elsewhere).
func validateRequiredFields(spec validation.ChallengeYamlSpec) []LintIssue {
	var issues []LintIssue
	v := reflect.ValueOf(spec)
	t := reflect.TypeOf(spec)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			continue
		}
		if strings.Contains(yamlTag, "omitempty") {
			continue // optional field
		}
		fieldName := strings.Split(yamlTag, ",")[0]
		fieldVal := v.Field(i)

		switch field.Type.Kind() {
		case reflect.String:
			if fieldVal.String() == "" {
				issues = append(issues, LintIssue{
					Field:    fieldName,
					Severity: SeverityError,
					Message:  fmt.Sprintf("required field %q is missing or empty", fieldName),
				})
			}
		case reflect.Int:
			if fieldVal.Int() <= 0 {
				issues = append(issues, LintIssue{
					Field:    fieldName,
					Severity: SeverityError,
					Message:  fmt.Sprintf("required field %q must be a positive number", fieldName),
				})
			}
			// Slices (Objectives) are validated separately — skip here.
		}
	}
	return issues
}
