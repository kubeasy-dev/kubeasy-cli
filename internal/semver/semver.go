// Package semver provides lightweight semantic version comparison utilities.
// It handles versions of the form vMAJOR.MINOR.PATCH with optional leading 'v'.
package semver

import "strings"

// Normalize trims a leading 'v' and strips pre-release/build metadata.
// Example: "v2.3.1-rc.1" → "2.3.1"
func Normalize(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	return v
}

// Compare compares two semantic versions (major.minor.patch).
// Returns -1 if a < b, 0 if equal, 1 if a > b.
// Inputs should be normalized first with Normalize.
func Compare(a, b string) int {
	as := splitToInt3(a)
	bs := splitToInt3(b)
	for i := range 3 {
		if as[i] < bs[i] {
			return -1
		}
		if as[i] > bs[i] {
			return 1
		}
	}
	return 0
}

// IsPreRelease returns true for non-semver version strings like "dev" or "nightly-abc1234".
// Strings starting with a non-digit (after stripping a leading 'v') are considered pre-release.
// Legitimate semver pre-releases like "2.7.0-rc.1" start with a digit and return false.
func IsPreRelease(v string) bool {
	v = strings.TrimSpace(strings.TrimPrefix(v, "v"))
	return len(v) == 0 || v[0] < '0' || v[0] > '9'
}

// splitToInt3 splits a dotted version string into a [3]int array.
func splitToInt3(v string) [3]int {
	var out [3]int
	parts := strings.Split(v, ".")
	for i := 0; i < len(parts) && i < 3; i++ {
		n := 0
		for _, ch := range parts[i] {
			if ch < '0' || ch > '9' {
				break
			}
			n = n*10 + int(ch-'0')
		}
		out[i] = n // #nosec G602 - i is bounded by loop condition i < 3
	}
	return out
}
