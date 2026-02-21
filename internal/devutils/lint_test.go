package devutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeChallengeFile(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "challenge.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestLintChallengeFile_Valid(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "manifests"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifests", "deploy.yaml"), []byte("apiVersion: v1"), 0o600))

	path := writeChallengeFile(t, dir, `
title: "Test Challenge"
type: "fix"
theme: "networking"
difficulty: "easy"
estimatedTime: 15
description: |
  Something is broken.
initialSituation: |
  A pod is running.
objective: |
  Fix it.
objectives:
  - key: pod-ready
    title: "Pod Ready"
    description: "Pod must be ready"
    order: 1
    type: condition
    spec:
      target:
        kind: Pod
        labelSelector:
          app: test
      checks:
        - type: Ready
          status: "True"
`)

	issues, err := LintChallengeFile(path)
	require.NoError(t, err)

	errors := filterBySeverity(issues, SeverityError)
	assert.Empty(t, errors, "expected no errors, got: %v", errors)
}

func TestLintChallengeFile_MissingFields(t *testing.T) {
	dir := t.TempDir()
	path := writeChallengeFile(t, dir, `
title: ""
estimatedTime: 0
`)

	issues, err := LintChallengeFile(path)
	require.NoError(t, err)

	errors := filterBySeverity(issues, SeverityError)
	assert.True(t, len(errors) > 0, "expected errors for missing fields")

	fields := make(map[string]bool)
	for _, issue := range errors {
		fields[issue.Field] = true
	}
	assert.True(t, fields["title"], "expected error for empty title")
	assert.True(t, fields["description"], "expected error for missing description")
	assert.True(t, fields["estimatedTime"], "expected error for estimatedTime <= 0")
}

func TestLintChallengeFile_InvalidDifficulty(t *testing.T) {
	dir := t.TempDir()
	path := writeChallengeFile(t, dir, `
title: "Test"
type: "fix"
theme: "networking"
difficulty: "extreme"
estimatedTime: 15
description: "desc"
initialSituation: "sit"
objective: "obj"
objectives: []
`)

	issues, err := LintChallengeFile(path)
	require.NoError(t, err)

	found := false
	for _, issue := range issues {
		if issue.Field == "difficulty" && issue.Severity == SeverityError {
			found = true
			break
		}
	}
	assert.True(t, found, "expected error for invalid difficulty")
}

func TestLintChallengeFile_InvalidType(t *testing.T) {
	dir := t.TempDir()
	path := writeChallengeFile(t, dir, `
title: "Test"
type: "destroy"
theme: "networking"
difficulty: "easy"
estimatedTime: 15
description: "desc"
initialSituation: "sit"
objective: "obj"
objectives: []
`)

	issues, err := LintChallengeFile(path)
	require.NoError(t, err)

	found := false
	for _, issue := range issues {
		if issue.Field == "type" && issue.Severity == SeverityError {
			found = true
			break
		}
	}
	assert.True(t, found, "expected error for invalid type")
}

func TestLintChallengeFile_DuplicateKeys(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "manifests"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifests", "deploy.yaml"), []byte("apiVersion: v1"), 0o600))

	path := writeChallengeFile(t, dir, `
title: "Test"
type: "fix"
theme: "networking"
difficulty: "easy"
estimatedTime: 15
description: "desc"
initialSituation: "sit"
objective: "obj"
objectives:
  - key: pod-ready
    title: "A"
    order: 1
    type: condition
    spec:
      target:
        kind: Pod
        labelSelector:
          app: test
      checks:
        - type: Ready
          status: "True"
  - key: pod-ready
    title: "B"
    order: 2
    type: condition
    spec:
      target:
        kind: Pod
        labelSelector:
          app: test
      checks:
        - type: Ready
          status: "True"
`)

	issues, err := LintChallengeFile(path)
	require.NoError(t, err)

	found := false
	for _, issue := range issues {
		if issue.Field == "objectives" && issue.Severity == SeverityError {
			found = true
			break
		}
	}
	assert.True(t, found, "expected error for duplicate keys")
}

func TestLintChallengeFile_ManifestsDirMissing(t *testing.T) {
	dir := t.TempDir()
	path := writeChallengeFile(t, dir, `
title: "Test"
type: "fix"
theme: "networking"
difficulty: "easy"
estimatedTime: 15
description: "desc"
initialSituation: "sit"
objective: "obj"
objectives: []
`)

	issues, err := LintChallengeFile(path)
	require.NoError(t, err)

	warnings := filterBySeverity(issues, SeverityWarning)
	found := false
	for _, w := range warnings {
		if w.Field == "manifests/" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected warning for missing manifests/")
}

func filterBySeverity(issues []LintIssue, severity LintSeverity) []LintIssue {
	var result []LintIssue
	for _, issue := range issues {
		if issue.Severity == severity {
			result = append(result, issue)
		}
	}
	return result
}
