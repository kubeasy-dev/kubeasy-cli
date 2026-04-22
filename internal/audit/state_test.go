package audit

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndLoadTimestamp_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	before := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, SaveTimestamp("test-slug"))
	after := time.Now().UTC().Truncate(time.Second)

	ts, err := LoadTimestamp("test-slug")
	require.NoError(t, err)
	assert.False(t, ts.Before(before), "loaded timestamp should be >= before")
	assert.False(t, ts.After(after), "loaded timestamp should be <= after")
}

func TestLoadTimestamp_MissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	ts, err := LoadTimestamp("nonexistent-slug")
	assert.Error(t, err)
	assert.True(t, ts.IsZero(), "zero time expected on missing file")
}

func TestClearState_RemovesDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	require.NoError(t, SaveTimestamp("test-slug"))
	stateDir := GetStateDir("test-slug")
	_, err := os.Stat(stateDir)
	require.NoError(t, err, "state dir should exist after SaveTimestamp")

	require.NoError(t, ClearState("test-slug"))
	_, err = os.Stat(stateDir)
	assert.True(t, os.IsNotExist(err), "state dir should be removed after ClearState")
}

func TestGetStateDir_ContainsSlug(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	assert.Equal(t, filepath.Join(dir, ".kubeasy", "state", "my-slug"), GetStateDir("my-slug"))
}
