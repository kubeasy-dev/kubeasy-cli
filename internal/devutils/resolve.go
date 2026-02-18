package devutils

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveLocalChallengeDir finds the local challenge directory for a given slug.
// It checks: dirFlag override, ./<slug>/, and whether cwd is the challenge dir.
func ResolveLocalChallengeDir(slug string, dirFlag string) (string, error) {
	if dirFlag != "" {
		absDir, err := filepath.Abs(dirFlag)
		if err != nil {
			return "", fmt.Errorf("failed to resolve path: %w", err)
		}
		if _, err := os.Stat(filepath.Join(absDir, "challenge.yaml")); err != nil {
			return "", fmt.Errorf("challenge.yaml not found in %s", absDir)
		}
		return absDir, nil
	}

	// Try ./<slug>/
	candidate := filepath.Join(".", slug)
	if _, err := os.Stat(filepath.Join(candidate, "challenge.yaml")); err == nil {
		return filepath.Abs(candidate)
	}

	// Try current directory (user is inside the challenge dir)
	if _, err := os.Stat(filepath.Join(".", "challenge.yaml")); err == nil {
		return filepath.Abs(".")
	}

	return "", fmt.Errorf("could not find challenge directory for '%s'. Use --dir to specify the path", slug)
}
