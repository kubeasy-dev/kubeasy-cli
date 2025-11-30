package kube

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestGetKubeConfigPath_Default(t *testing.T) {
	// Clear KUBECONFIG env var
	oldKubeConfig := os.Getenv("KUBECONFIG")
	defer func() { _ = os.Setenv("KUBECONFIG", oldKubeConfig) }()
	_ = os.Unsetenv("KUBECONFIG")

	// Execute
	path := GetKubeConfigPath()

	// Assert
	assert.NotEmpty(t, path)
	assert.Contains(t, path, ".kube")
	assert.Contains(t, path, "config")
}

func TestGetKubeConfigPath_FromEnv(t *testing.T) {
	// Set KUBECONFIG env var
	testPath := "/custom/path/to/config"
	oldKubeConfig := os.Getenv("KUBECONFIG")
	defer func() { _ = os.Setenv("KUBECONFIG", oldKubeConfig) }()
	_ = os.Setenv("KUBECONFIG", testPath)

	// Execute
	path := GetKubeConfigPath()

	// Assert
	assert.Equal(t, testPath, path)
}

func TestGetDefaultKubeconfigPath(t *testing.T) {
	// Execute
	path := GetDefaultKubeconfigPath()

	// Assert - should return a path even if file doesn't exist
	assert.NotEmpty(t, path)
	assert.Contains(t, path, ".kube")
	assert.Contains(t, path, "config")
}

func TestSetNamespaceForContext_Success(t *testing.T) {
	// Create a temporary kubeconfig file
	tmpDir := t.TempDir()
	kubeDir := filepath.Join(tmpDir, ".kube")
	err := os.MkdirAll(kubeDir, 0755)
	require.NoError(t, err)
	kubeconfigPath := filepath.Join(kubeDir, "config")

	// Create a minimal kubeconfig with a test context
	config := clientcmdapi.NewConfig()
	config.Clusters["test-cluster"] = &clientcmdapi.Cluster{
		Server: "https://localhost:6443",
	}
	config.AuthInfos["test-user"] = &clientcmdapi.AuthInfo{
		Token: "test-token",
	}
	config.Contexts["test-context"] = &clientcmdapi.Context{
		Cluster:   "test-cluster",
		AuthInfo:  "test-user",
		Namespace: "default",
	}
	config.CurrentContext = "test-context"

	err = clientcmd.WriteToFile(*config, kubeconfigPath)
	require.NoError(t, err)

	// Override HOME to use our temp directory
	oldHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", oldHome) }()

	// Execute
	err = SetNamespaceForContext("test-context", "test-namespace")

	// Assert
	require.NoError(t, err)

	// Verify the kubeconfig was updated
	loadedConfig, err := clientcmd.LoadFromFile(kubeconfigPath)
	require.NoError(t, err)
	assert.Equal(t, "test-namespace", loadedConfig.Contexts["test-context"].Namespace)
	assert.Equal(t, "test-context", loadedConfig.CurrentContext)
}

func TestSetNamespaceForContext_ContextNotFound(t *testing.T) {
	// Create a temporary kubeconfig file
	tmpDir := t.TempDir()
	kubeDir := filepath.Join(tmpDir, ".kube")
	err := os.MkdirAll(kubeDir, 0755)
	require.NoError(t, err)
	kubeconfigPath := filepath.Join(kubeDir, "config")

	// Create a minimal kubeconfig WITHOUT the context we're looking for
	config := clientcmdapi.NewConfig()
	config.Clusters["test-cluster"] = &clientcmdapi.Cluster{
		Server: "https://localhost:6443",
	}
	config.AuthInfos["test-user"] = &clientcmdapi.AuthInfo{
		Token: "test-token",
	}
	config.Contexts["other-context"] = &clientcmdapi.Context{
		Cluster:  "test-cluster",
		AuthInfo: "test-user",
	}
	config.CurrentContext = "other-context"

	err = clientcmd.WriteToFile(*config, kubeconfigPath)
	require.NoError(t, err)

	// Override HOME to use our temp file
	oldHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", oldHome) }()

	// Execute
	err = SetNamespaceForContext("nonexistent-context", "test-namespace")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context 'nonexistent-context' not found")
}

func TestSetNamespaceForContext_InvalidKubeconfig(t *testing.T) {
	// Create a temporary directory without a kubeconfig file
	tmpDir := t.TempDir()

	// Override HOME to use our temp dir (which has no config file)
	oldHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", oldHome) }()

	// Execute - this should fail because the config file doesn't exist
	err := SetNamespaceForContext("test-context", "test-namespace")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load kubeconfig")
}

func TestSetNamespaceForContext_UpdatesCurrentContext(t *testing.T) {
	// Create a temporary kubeconfig file with multiple contexts
	tmpDir := t.TempDir()
	kubeDir := filepath.Join(tmpDir, ".kube")
	err := os.MkdirAll(kubeDir, 0755)
	require.NoError(t, err)
	kubeconfigPath := filepath.Join(kubeDir, "config")

	config := clientcmdapi.NewConfig()
	config.Clusters["cluster1"] = &clientcmdapi.Cluster{Server: "https://localhost:6443"}
	config.Clusters["cluster2"] = &clientcmdapi.Cluster{Server: "https://localhost:6444"}
	config.AuthInfos["user1"] = &clientcmdapi.AuthInfo{Token: "token1"}

	config.Contexts["context1"] = &clientcmdapi.Context{
		Cluster:   "cluster1",
		AuthInfo:  "user1",
		Namespace: "default",
	}
	config.Contexts["context2"] = &clientcmdapi.Context{
		Cluster:   "cluster2",
		AuthInfo:  "user1",
		Namespace: "default",
	}
	config.CurrentContext = "context1" // Start with context1 as current

	err = clientcmd.WriteToFile(*config, kubeconfigPath)
	require.NoError(t, err)

	// Override HOME
	oldHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", oldHome) }()

	// Execute - set namespace for context2, which should also make it current
	err = SetNamespaceForContext("context2", "new-namespace")

	// Assert
	require.NoError(t, err)

	// Verify both namespace and current-context were updated
	loadedConfig, err := clientcmd.LoadFromFile(kubeconfigPath)
	require.NoError(t, err)
	assert.Equal(t, "new-namespace", loadedConfig.Contexts["context2"].Namespace)
	assert.Equal(t, "context2", loadedConfig.CurrentContext)
}

func TestGetRestConfig_ValidContext(t *testing.T) {
	// This test requires a valid kubeconfig with the kubeasy context
	// We'll skip it if the context doesn't exist

	// Try to get the config
	config, err := GetRestConfig()

	// If we get an error, check if it's because the context doesn't exist
	if err != nil {
		errMsg := err.Error()
		if contains(errMsg, constants.KubeasyClusterContext) || contains(errMsg, "context") {
			t.Skip("Skipping test: kubeasy context not found in kubeconfig")
		}
		// If it's a different error, fail the test
		t.Fatalf("Unexpected error: %v", err)
	}

	// If we got here, the context exists and config should be valid
	assert.NotNil(t, config)
	assert.NotEmpty(t, config.Host)
}

func TestGetRestConfig_WithTempKubeconfig(t *testing.T) {
	// Create a temporary kubeconfig file with kubeasy context
	tmpDir := t.TempDir()
	kubeconfigPath := filepath.Join(tmpDir, "config")

	config := clientcmdapi.NewConfig()
	config.Clusters[constants.KubeasyClusterContext] = &clientcmdapi.Cluster{
		Server: "https://localhost:6443",
	}
	config.AuthInfos["kubeasy-user"] = &clientcmdapi.AuthInfo{
		Token: "test-token",
	}
	config.Contexts[constants.KubeasyClusterContext] = &clientcmdapi.Context{
		Cluster:   constants.KubeasyClusterContext,
		AuthInfo:  "kubeasy-user",
		Namespace: "default",
	}
	config.CurrentContext = constants.KubeasyClusterContext

	err := clientcmd.WriteToFile(*config, kubeconfigPath)
	require.NoError(t, err)

	// Override KUBECONFIG to use our temp file
	oldKubeConfig := os.Getenv("KUBECONFIG")
	_ = os.Setenv("KUBECONFIG", kubeconfigPath)
	defer func() { _ = os.Setenv("KUBECONFIG", oldKubeConfig) }()

	// Execute
	restConfig, err := GetRestConfig()

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, restConfig)
	assert.Equal(t, "https://localhost:6443", restConfig.Host)
	assert.Equal(t, "test-token", restConfig.BearerToken)
}
