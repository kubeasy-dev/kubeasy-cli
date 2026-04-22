package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKindClusterConfig_AuditExtraMounts(t *testing.T) {
	cfg := kindClusterConfig()
	require.Len(t, cfg.Nodes, 1)
	node := cfg.Nodes[0]

	assert.Len(t, node.ExtraMounts, 2, "control-plane node must have exactly 2 ExtraMounts")

	// Policy file mount
	policyMount := node.ExtraMounts[0]
	assert.Equal(t, "/etc/kubernetes/audit-policy.yaml", policyMount.ContainerPath)
	assert.True(t, policyMount.Readonly)

	// Log directory mount
	logMount := node.ExtraMounts[1]
	assert.Equal(t, "/var/log/kubernetes/audit", logMount.ContainerPath)
	assert.False(t, logMount.Readonly)
}

func TestKindClusterConfig_AuditKubeadmConfigPatches(t *testing.T) {
	cfg := kindClusterConfig()
	require.Len(t, cfg.Nodes, 1)
	node := cfg.Nodes[0]

	require.Len(t, node.KubeadmConfigPatches, 1, "control-plane node must have exactly 1 KubeadmConfigPatches entry")
	patch := node.KubeadmConfigPatches[0]
	assert.Contains(t, patch, "audit-policy-file")
	assert.Contains(t, patch, "audit-log-path")
	assert.True(t, strings.HasPrefix(strings.TrimSpace(patch), "kind: ClusterConfiguration"))
}
