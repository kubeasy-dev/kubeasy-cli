package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidateChallengeSlug verifies that validateChallengeSlug accepts valid slugs
// and rejects invalid ones with appropriate error messages.
func TestValidateChallengeSlug(t *testing.T) {
	tests := []struct {
		name    string
		slug    string
		wantErr bool
	}{
		// Valid slugs
		{name: "valid_basic", slug: "pod-evicted", wantErr: false},
		{name: "valid_two_words", slug: "basic-pod", wantErr: false},
		{name: "valid_with_numbers", slug: "my-challenge-123", wantErr: false},
		{name: "valid_alphanumeric", slug: "config-map-101", wantErr: false},
		{name: "valid_min_length", slug: "a1b", wantErr: false},

		// Invalid: uppercase letters
		{name: "invalid_uppercase", slug: "Pod-Evicted", wantErr: true},
		{name: "invalid_all_upper", slug: "POD-EVICTED", wantErr: true},

		// Invalid: spaces
		{name: "invalid_spaces", slug: "pod evicted", wantErr: true},

		// Invalid: length
		{name: "invalid_too_short", slug: "ab", wantErr: true},
		{name: "invalid_empty", slug: "", wantErr: true},

		// Invalid: hyphens in wrong position
		{name: "invalid_leading_hyphen", slug: "-pod-evicted", wantErr: true},
		{name: "invalid_trailing_hyphen", slug: "pod-evicted-", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validateChallengeSlug(tc.slug)
			if tc.wantErr {
				assert.Error(t, err, "expected error for slug %q", tc.slug)
			} else {
				assert.NoError(t, err, "expected no error for slug %q", tc.slug)
			}
		})
	}
}
