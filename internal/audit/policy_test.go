package audit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestEnsureAuditPolicy_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	require.NoError(t, EnsureAuditPolicy())

	data, err := os.ReadFile(GetAuditPolicyPath())
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

func TestEnsureAuditPolicy_Idempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	require.NoError(t, EnsureAuditPolicy())
	require.NoError(t, EnsureAuditPolicy(), "second call should not fail")

	data, err := os.ReadFile(GetAuditPolicyPath())
	require.NoError(t, err)
	assert.Equal(t, AuditPolicyYAML, string(data))
}

func TestAuditPolicyYAML_ValidYAML(t *testing.T) {
	var out interface{}
	require.NoError(t, yaml.Unmarshal([]byte(AuditPolicyYAML), &out), "AuditPolicyYAML must be valid YAML")
}

func TestGetAuditDir_ContainsAudit(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	assert.Equal(t, filepath.Join(dir, ".kubeasy", "audit"), GetAuditDir())
}
