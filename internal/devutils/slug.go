package devutils

import (
	"regexp"
	"strings"
)

// GenerateSlug converts a challenge name to a valid slug.
// Lowercase, spaces/underscores become hyphens, strips invalid characters, collapses hyphens.
func GenerateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")

	// Strip non-alphanumeric/non-hyphen characters
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	slug = reg.ReplaceAllString(slug, "")

	// Collapse consecutive hyphens
	reg = regexp.MustCompile(`-{2,}`)
	slug = reg.ReplaceAllString(slug, "-")

	// Trim leading/trailing hyphens
	slug = strings.Trim(slug, "-")

	return slug
}
