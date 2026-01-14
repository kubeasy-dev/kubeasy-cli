package fieldpath

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	// Regex patterns for parsing field path segments
	// Matches: field, field[0], field[key=value], field[] (empty brackets caught later)
	segmentPattern   = regexp.MustCompile(`^([a-zA-Z][a-zA-Z0-9]*)((?:\[[^\]]*\])*)$`)
	arrayPattern     = regexp.MustCompile(`\[([^\]]*)\]`)
	filterPattern    = regexp.MustCompile(`^([a-zA-Z][a-zA-Z0-9]*)=(.+)$`)
	fieldNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*$`)
)

const (
	// MaxPathLength limits the total length of a field path to prevent DoS
	MaxPathLength = 1000
	// MaxPathDepth limits the nesting depth to prevent stack overflow
	MaxPathDepth = 50
)

// Parse parses a field path string into a sequence of tokens.
//
// The input path is automatically prefixed with "status." before parsing.
// NOTE: This hardcoded prefix makes the parser specific to Kubernetes status fields.
// This is intentional for the current use case (status validation), but limits reusability.
// If you need to parse paths in other contexts, consider creating a separate function.
//
// Examples:
//   - "readyReplicas" -> []PathToken{FieldToken{Name: "status"}, FieldToken{Name: "readyReplicas"}}
//   - "containerStatuses[0].restartCount" -> tokens for status, containerStatuses, [0], restartCount
//   - "conditions[type=Ready].status" -> tokens for status, conditions, [type=Ready], status
func Parse(path string) ([]PathToken, error) {
	if path == "" {
		return nil, fmt.Errorf("field path cannot be empty")
	}

	// Validate path length to prevent DoS attacks
	if len(path) > MaxPathLength {
		return nil, fmt.Errorf("field path exceeds maximum length of %d characters", MaxPathLength)
	}

	// Automatically prefix with "status."
	fullPath := "status." + path

	// Split by dots, but protect content inside brackets from being split
	segments, err := splitPath(fullPath)
	if err != nil {
		return nil, err
	}

	// Validate path depth to prevent stack overflow
	if len(segments) > MaxPathDepth {
		return nil, fmt.Errorf("field path exceeds maximum depth of %d levels", MaxPathDepth)
	}

	var tokens []PathToken

	for i, segment := range segments {
		if segment == "" {
			return nil, fmt.Errorf("empty segment at position %d in path %q", i, path)
		}

		// Parse segment with potential array accessors
		segmentTokens, err := parseSegment(segment, i, path)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, segmentTokens...)
	}

	return tokens, nil
}

// splitPath splits a path by dots, but ignores dots inside brackets.
// Returns an error if brackets are mismatched.
// Example: "field[key=value.with.dots].nested" -> ["field[key=value.with.dots]", "nested"]
func splitPath(path string) ([]string, error) {
	var segments []string
	var current strings.Builder
	bracketDepth := 0

	for _, ch := range path {
		switch ch {
		case '[':
			bracketDepth++
			current.WriteRune(ch)
		case ']':
			bracketDepth--
			// Check for negative depth (more closing than opening brackets)
			if bracketDepth < 0 {
				return nil, fmt.Errorf("mismatched brackets: extra closing bracket in path %q", path)
			}
			current.WriteRune(ch)
		case '.':
			if bracketDepth == 0 {
				// We're outside brackets, this is a real separator
				segments = append(segments, current.String())
				current.Reset()
			} else {
				// We're inside brackets, keep the dot as part of the segment
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	// Validate that all brackets were closed
	if bracketDepth != 0 {
		return nil, fmt.Errorf("mismatched brackets: %d unclosed bracket(s) in path %q", bracketDepth, path)
	}

	// Add the last segment
	if current.Len() > 0 {
		segments = append(segments, current.String())
	}

	return segments, nil
}

// parseSegment parses a single segment that may include array accessors
// Examples:
//   - "field" -> [FieldToken{Name: "field"}]
//   - "field[0]" -> [FieldToken{Name: "field"}, ArrayIndexToken{Index: 0}]
//   - "field[type=Ready]" -> [FieldToken{Name: "field"}, ArrayFilterToken{...}]
//   - "field[0][1]" -> [FieldToken{Name: "field"}, ArrayIndexToken{Index: 0}, ArrayIndexToken{Index: 1}]
func parseSegment(segment string, position int, originalPath string) ([]PathToken, error) {
	// Check if segment has array accessors
	matches := segmentPattern.FindStringSubmatch(segment)
	if matches == nil {
		// Simple field without brackets
		if !isValidFieldName(segment) {
			return nil, fmt.Errorf("invalid field name %q at position %d in path %q", segment, position, originalPath)
		}
		return []PathToken{FieldToken{Name: segment}}, nil
	}

	fieldName := matches[1]
	arrayPart := matches[2]

	tokens := []PathToken{FieldToken{Name: fieldName}}

	// Parse array accessors
	if arrayPart != "" {
		arrayMatches := arrayPattern.FindAllStringSubmatch(arrayPart, -1)
		for _, arrayMatch := range arrayMatches {
			accessor := arrayMatch[1]
			arrayToken, err := parseArrayAccessor(accessor, position, originalPath)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, arrayToken)
		}
	}

	return tokens, nil
}

// parseArrayAccessor parses the content inside brackets [...]
// Returns either ArrayIndexToken or ArrayFilterToken
func parseArrayAccessor(accessor string, position int, originalPath string) (PathToken, error) {
	// Check for empty accessor
	if accessor == "" {
		return nil, fmt.Errorf("invalid array accessor: empty brackets at position %d in path %q", position, originalPath)
	}

	// Try to parse as integer index
	if index, err := strconv.Atoi(accessor); err == nil {
		if index < 0 {
			return nil, fmt.Errorf("array index must be non-negative, got %d at position %d in path %q", index, position, originalPath)
		}
		return ArrayIndexToken{Index: index}, nil
	}

	// Try to parse as filter (key=value)
	filterMatches := filterPattern.FindStringSubmatch(accessor)
	if filterMatches == nil {
		// Check if it looks like an attempted filter (contains '=')
		if strings.Contains(accessor, "=") {
			return nil, fmt.Errorf("invalid array filter %q at position %d in path %q (filter must be in format 'key=value' with non-empty value)", accessor, position, originalPath)
		}
		return nil, fmt.Errorf("invalid array accessor %q at position %d in path %q (expected integer index or key=value filter)", accessor, position, originalPath)
	}

	filterField := filterMatches[1]
	filterValue := filterMatches[2]

	return ArrayFilterToken{
		FilterField: filterField,
		FilterValue: filterValue,
	}, nil
}

// isValidFieldName checks if a string is a valid Go field name.
// Must start with a letter, followed by letters or digits.
func isValidFieldName(name string) bool {
	if name == "" {
		return false
	}
	return fieldNamePattern.MatchString(name)
}
