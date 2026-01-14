package validation

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/validation/fieldpath"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

// kindToStatusType maps Kubernetes resource kinds to their Go Status types.
// Only includes resources that have a Status field.
// Used for compile-time validation of field paths.
var kindToStatusType = map[string]reflect.Type{
	// Core v1
	"Pod":                   reflect.TypeOf(corev1.PodStatus{}),
	"Service":               reflect.TypeOf(corev1.ServiceStatus{}),
	"PersistentVolumeClaim": reflect.TypeOf(corev1.PersistentVolumeClaimStatus{}),
	"PersistentVolume":      reflect.TypeOf(corev1.PersistentVolumeStatus{}),
	"Node":                  reflect.TypeOf(corev1.NodeStatus{}),
	"Namespace":             reflect.TypeOf(corev1.NamespaceStatus{}),
	"ReplicationController": reflect.TypeOf(corev1.ReplicationControllerStatus{}),

	// Apps v1
	"Deployment":  reflect.TypeOf(appsv1.DeploymentStatus{}),
	"StatefulSet": reflect.TypeOf(appsv1.StatefulSetStatus{}),
	"DaemonSet":   reflect.TypeOf(appsv1.DaemonSetStatus{}),
	"ReplicaSet":  reflect.TypeOf(appsv1.ReplicaSetStatus{}),

	// Batch v1
	"Job":     reflect.TypeOf(batchv1.JobStatus{}),
	"CronJob": reflect.TypeOf(batchv1.CronJobStatus{}),

	// Networking v1
	"Ingress": reflect.TypeOf(networkingv1.IngressStatus{}),
}

// ValidateFieldPath validates that a field path exists in the given Kind's Status.
// The field path should NOT include "status." prefix (it's automatically added).
//
// Examples:
//   - ValidateFieldPath("Deployment", "readyReplicas") -> nil
//   - ValidateFieldPath("Pod", "containerStatuses[0].restartCount") -> nil
//   - ValidateFieldPath("Deployment", "unknownField") -> error with available fields
//
// Returns nil if the field path is valid, or an error with helpful suggestions.
func ValidateFieldPath(kind string, fieldPath string) error {
	// Get status type for kind
	statusType, ok := kindToStatusType[kind]
	if !ok {
		return fmt.Errorf("unsupported kind %q for field validation (supported: %s)",
			kind, listSupportedKinds())
	}

	// Parse field path to tokens
	tokens, err := fieldpath.Parse(fieldPath)
	if err != nil {
		return fmt.Errorf("invalid field path syntax: %w", err)
	}

	// Skip the first token ("status") since we start from the Status type
	if len(tokens) > 0 {
		if ft, ok := tokens[0].(fieldpath.FieldToken); ok && ft.Name == "status" {
			tokens = tokens[1:]
		}
	}

	// Navigate type structure using reflection
	return validateTokensAgainstType(statusType, tokens, fieldPath)
}

// validateTokensAgainstType navigates the type structure using the parsed tokens.
// Returns nil if the path is valid, or an error with the first invalid segment.
func validateTokensAgainstType(t reflect.Type, tokens []fieldpath.PathToken, originalPath string) error {
	currentType := t

	for i, token := range tokens {
		switch tok := token.(type) {
		case fieldpath.FieldToken:
			// Handle pointer types
			if currentType.Kind() == reflect.Ptr {
				currentType = currentType.Elem()
			}

			// Must be a struct to access fields
			if currentType.Kind() != reflect.Struct {
				return fmt.Errorf("cannot access field %q on non-struct type %s in path %q",
					tok.Name, currentType.Kind(), originalPath)
			}

			// Find the field (case-insensitive match with JSON tags)
			field, found := findStructField(currentType, tok.Name)
			if !found {
				return fmt.Errorf("field %q not found in %s (path: %q)\navailable fields: %s",
					tok.Name, currentType.Name(), originalPath, listAvailableFields(currentType))
			}

			currentType = field.Type

		case fieldpath.ArrayIndexToken:
			// Handle pointer types
			if currentType.Kind() == reflect.Ptr {
				currentType = currentType.Elem()
			}

			// Must be a slice or array
			if currentType.Kind() != reflect.Slice && currentType.Kind() != reflect.Array {
				return fmt.Errorf("cannot use array index [%d] on non-array type %s in path %q",
					tok.Index, currentType.Kind(), originalPath)
			}

			// Get element type
			currentType = currentType.Elem()

		case fieldpath.ArrayFilterToken:
			// Handle pointer types
			if currentType.Kind() == reflect.Ptr {
				currentType = currentType.Elem()
			}

			// Must be a slice or array
			if currentType.Kind() != reflect.Slice && currentType.Kind() != reflect.Array {
				return fmt.Errorf("cannot use array filter [%s=%s] on non-array type %s in path %q",
					tok.FilterField, tok.FilterValue, currentType.Kind(), originalPath)
			}

			// Get element type and verify filter field exists
			elemType := currentType.Elem()
			if elemType.Kind() == reflect.Ptr {
				elemType = elemType.Elem()
			}

			if elemType.Kind() == reflect.Struct {
				_, found := findStructField(elemType, tok.FilterField)
				if !found {
					return fmt.Errorf("filter field %q not found in array element type %s (path: %q)\navailable fields: %s",
						tok.FilterField, elemType.Name(), originalPath, listAvailableFields(elemType))
				}
			}

			currentType = elemType

		default:
			return fmt.Errorf("unknown token type at position %d in path %q", i, originalPath)
		}
	}

	return nil
}

// findStructField finds a field in a struct type by name.
// Matches against the JSON tag first (case-insensitive), then the field name.
func findStructField(t reflect.Type, name string) (reflect.StructField, bool) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return reflect.StructField{}, false
	}

	lowerName := strings.ToLower(name)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Check JSON tag first
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" {
			// Parse JSON tag (format: "name,omitempty" or "name")
			tagName := strings.Split(jsonTag, ",")[0]
			if tagName != "-" && strings.ToLower(tagName) == lowerName {
				return field, true
			}
		}

		// Fall back to field name (case-insensitive)
		if strings.ToLower(field.Name) == lowerName {
			return field, true
		}
	}

	return reflect.StructField{}, false
}

// listAvailableFields returns a comma-separated list of available field names for a struct type.
// Uses JSON tag names when available, otherwise the Go field name with first letter lowercased.
func listAvailableFields(t reflect.Type) string {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return "(not a struct)"
	}

	var fields []string

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get field name from JSON tag or use lowercase first letter
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" {
			tagName := strings.Split(jsonTag, ",")[0]
			if tagName != "" {
				fields = append(fields, tagName)
				continue
			}
		}

		// Use field name with lowercase first letter
		fields = append(fields, lowercaseFirst(field.Name))
	}

	sort.Strings(fields)
	return strings.Join(fields, ", ")
}

// listSupportedKinds returns a comma-separated list of supported Kubernetes kinds.
func listSupportedKinds() string {
	kinds := make([]string, 0, len(kindToStatusType))
	for kind := range kindToStatusType {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return strings.Join(kinds, ", ")
}

// lowercaseFirst returns the string with the first letter lowercased.
func lowercaseFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// capitalizeFirst returns the string with the first letter capitalized.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// GetSupportedKinds returns a list of all Kubernetes kinds that support field validation.
func GetSupportedKinds() []string {
	kinds := make([]string, 0, len(kindToStatusType))
	for kind := range kindToStatusType {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return kinds
}

// IsKindSupported checks if a Kubernetes kind supports field validation.
func IsKindSupported(kind string) bool {
	_, ok := kindToStatusType[kind]
	return ok
}
