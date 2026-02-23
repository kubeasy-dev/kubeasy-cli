package devutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateSlug(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple lowercase",
			input:    "my challenge",
			expected: "my-challenge",
		},
		{
			name:     "mixed case",
			input:    "My Challenge Name",
			expected: "my-challenge-name",
		},
		{
			name:     "underscores to hyphens",
			input:    "my_challenge_name",
			expected: "my-challenge-name",
		},
		{
			name:     "special characters stripped",
			input:    "my challenge! @#$%",
			expected: "my-challenge",
		},
		{
			name:     "consecutive hyphens collapsed",
			input:    "my---challenge",
			expected: "my-challenge",
		},
		{
			name:     "leading and trailing hyphens trimmed",
			input:    "  -my challenge-  ",
			expected: "my-challenge",
		},
		{
			name:     "mixed spaces underscores and special chars",
			input:    "Fix Pod_Evicted! (hard)",
			expected: "fix-pod-evicted-hard",
		},
		{
			name:     "already a valid slug",
			input:    "my-challenge",
			expected: "my-challenge",
		},
		{
			name:     "numbers preserved",
			input:    "Challenge 42 Test",
			expected: "challenge-42-test",
		},
		{
			name:     "only special characters",
			input:    "!@#$%^&*()",
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "unicode characters stripped",
			input:    "déploiement réseau",
			expected: "dploiement-rseau",
		},
		{
			name:     "tabs and newlines become hyphens via strip",
			input:    "my\tchallenge\nname",
			expected: "mychallengename",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateSlug(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
