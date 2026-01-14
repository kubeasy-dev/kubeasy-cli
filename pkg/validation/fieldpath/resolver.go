package fieldpath

import (
	"fmt"
	"sort"
	"strings"
)

// Resolve navigates through an object using the provided tokens and returns the final value.
// Returns (value, found, error).
// - value: the resolved value (nil if not found)
// - found: true if the path exists and was successfully resolved
// - error: non-nil if there was an error during resolution (type mismatch, out of bounds, etc.)
//
// Error Behavior:
// - Missing field: returns (nil, false, nil) - field doesn't exist, not an error
// - Type mismatch: returns (nil, false, error) - expected map/slice but got something else
// - Out of bounds: returns (nil, false, error) - array index out of range
// - Filter not found: returns (nil, false, error) - no array element matches filter
// This asymmetry is intentional: missing fields are common and expected, while type mismatches
// and out-of-bounds access indicate either incorrect paths or unexpected object structure.
func Resolve(obj map[string]interface{}, tokens []PathToken) (interface{}, bool, error) {
	if len(tokens) == 0 {
		return nil, false, fmt.Errorf("no tokens to resolve")
	}

	var current interface{} = obj

	for i, token := range tokens {
		switch t := token.(type) {
		case FieldToken:
			// Current must be a map
			currentMap, ok := current.(map[string]interface{})
			if !ok {
				return nil, false, fmt.Errorf("expected map at token %d (%s), got %T", i, t.Name, current)
			}

			// Access field with case-insensitive fallback
			// This tries three variations to handle JSON unmarshaling vs Go struct field names:
			// 1. Exact match (e.g., "readyReplicas")
			// 2. Lowercase first letter (e.g., "readyReplicas" -> "readyReplicas")
			// 3. Uppercase first letter (e.g., "readyReplicas" -> "ReadyReplicas")
			//
			// Performance note: In worst case, this performs 3 map lookups per field.
			// This is acceptable for typical Kubernetes object sizes, but could be optimized
			// if profiling shows it's a bottleneck (e.g., by caching or normalizing keys).
			//
			// Design choice: We prefer convenience (accepting either case) over performance.
			// Users typically write paths matching JSON fields (lowercase first), so the first
			// lookup usually succeeds.
			val, exists := currentMap[t.Name]
			if !exists {
				// Try with first letter lowercased (for JSON unmarshaling compatibility)
				altName := lowercaseFirst(t.Name)
				val, exists = currentMap[altName]
				if !exists {
					// Try with first letter capitalized (for Go struct field names)
					altName = capitalizeFirst(t.Name)
					val, exists = currentMap[altName]
					if !exists {
						return nil, false, nil // Field doesn't exist
					}
				}
			}

			current = val

		case ArrayIndexToken:
			// Current must be a slice
			currentSlice, ok := current.([]interface{})
			if !ok {
				return nil, false, fmt.Errorf("expected array at token %d (index %d), got %T", i, t.Index, current)
			}

			// Check bounds
			if t.Index >= len(currentSlice) {
				return nil, false, fmt.Errorf("array index %d out of bounds (length: %d) at token %d", t.Index, len(currentSlice), i)
			}

			current = currentSlice[t.Index]

		case ArrayFilterToken:
			// Current must be a slice
			currentSlice, ok := current.([]interface{})
			if !ok {
				return nil, false, fmt.Errorf("expected array at token %d (filter %s=%s), got %T", i, t.FilterField, t.FilterValue, current)
			}

			// Find matching element
			var found bool
			for _, elem := range currentSlice {
				elemMap, ok := elem.(map[string]interface{})
				if !ok {
					continue // Skip non-map elements
				}

				// Check if filter field matches filter value
				fieldVal, exists := elemMap[t.FilterField]
				if !exists {
					// Try capitalized version
					fieldVal, exists = elemMap[capitalizeFirst(t.FilterField)]
					if !exists {
						// Try lowercased version
						fieldVal, exists = elemMap[lowercaseFirst(t.FilterField)]
					}
				}

				if exists && compareFilterValue(fieldVal, t.FilterValue) {
					current = elemMap
					found = true
					break
				}
			}

			if !found {
				// Get available values for helpful error message
				availableValues := getAvailableFilterValues(currentSlice, t.FilterField)
				return nil, false, fmt.Errorf("no array element found with %s=%s at token %d (available values: %v)",
					t.FilterField, t.FilterValue, i, availableValues)
			}

		default:
			return nil, false, fmt.Errorf("unknown token type at position %d", i)
		}
	}

	return current, true, nil
}

// Get is a convenience function that parses a path and resolves it in one call.
// It automatically prefixes the path with "status." as per the Parse function.
func Get(obj map[string]interface{}, path string) (interface{}, bool, error) {
	tokens, err := Parse(path)
	if err != nil {
		return nil, false, fmt.Errorf("failed to parse path: %w", err)
	}

	return Resolve(obj, tokens)
}

// capitalizeFirst capitalizes the first letter of a string
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// lowercaseFirst lowercases the first letter of a string
func lowercaseFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// getAvailableFilterValues extracts all unique values for a filter field from a slice.
// Used for helpful error messages. Returns ["none"] for nil or empty slices.
func getAvailableFilterValues(slice []interface{}, filterField string) []string {
	// Defensive: handle nil slice (though caller should prevent this)
	if slice == nil {
		return []string{"none"}
	}

	values := make(map[string]bool)

	for _, elem := range slice {
		elemMap, ok := elem.(map[string]interface{})
		if !ok {
			continue
		}

		// Try different case variations
		var fieldVal interface{}
		var exists bool

		fieldVal, exists = elemMap[filterField]
		if !exists {
			fieldVal, exists = elemMap[capitalizeFirst(filterField)]
		}
		if !exists {
			fieldVal, exists = elemMap[lowercaseFirst(filterField)]
		}

		if exists {
			values[fmt.Sprintf("%v", fieldVal)] = true
		}
	}

	// Convert to sorted slice
	result := make([]string, 0, len(values))
	for v := range values {
		result = append(result, v)
	}

	if len(result) == 0 {
		return []string{"none"}
	}

	sort.Strings(result)
	return result
}

// compareFilterValue compares a field value against the filter value string in a type-aware manner.
// This provides more predictable behavior than simple string conversion.
//
// Comparison rules:
// - Strings: direct comparison
// - Numbers: convert both to strings and compare
// - Booleans: convert to "true"/"false" strings
// - Nil: only matches empty filter value ""
func compareFilterValue(fieldVal interface{}, filterValue string) bool {
	if fieldVal == nil {
		return filterValue == ""
	}

	switch v := fieldVal.(type) {
	case string:
		return v == filterValue
	case bool:
		// Compare boolean as "true" or "false" string
		boolStr := "false"
		if v {
			boolStr = "true"
		}
		return boolStr == filterValue
	case int, int8, int16, int32, int64:
		// All integer types
		return fmt.Sprintf("%d", v) == filterValue
	case uint, uint8, uint16, uint32, uint64:
		// All unsigned integer types
		return fmt.Sprintf("%d", v) == filterValue
	case float32, float64:
		// Floating point numbers
		return fmt.Sprintf("%v", v) == filterValue
	default:
		// Fallback to string conversion for other types
		return fmt.Sprintf("%v", v) == filterValue
	}
}
