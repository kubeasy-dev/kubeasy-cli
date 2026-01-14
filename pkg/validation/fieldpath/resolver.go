package fieldpath

import (
	"fmt"
	"strings"
)

// Resolve navigates through an object using the provided tokens and returns the final value.
// Returns (value, found, error).
// - value: the resolved value (nil if not found)
// - found: true if the path exists and was successfully resolved
// - error: non-nil if there was an error during resolution (type mismatch, out of bounds, etc.)
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

			// Access field (try exact case first, then try capitalized for JSON compatibility)
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

				if exists && fmt.Sprintf("%v", fieldVal) == t.FilterValue {
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

	return result
}
