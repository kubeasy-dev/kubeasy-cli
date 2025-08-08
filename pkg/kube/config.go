package kube

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/constants"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/logger"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// GetKubeConfigPath returns the path to the kubeconfig file
func GetKubeConfigPath() string {
	if envPath := os.Getenv("KUBECONFIG"); envPath != "" {
		return envPath
	}
	return filepath.Join(homedir.HomeDir(), ".kube", "config")
}

// GetRestConfig returns the Kubernetes REST config for the Kubeasy context
func GetRestConfig() (*rest.Config, error) {
	// Use the default kubeconfig location
	kubeConfigPath := GetKubeConfigPath()

	// Load the kubeconfig file with context override
	configLoadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfigPath}
	configOverrides := &clientcmd.ConfigOverrides{
		CurrentContext: constants.KubeasyClusterContext,
	}

	// Create the client configuration
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		configLoadingRules,
		configOverrides,
	).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("error building kubeconfig with context %s: %w", constants.KubeasyClusterContext, err)
	}

	return config, nil
}

// GetDefaultKubeconfigPath returns the default path for the kubeconfig file.
func GetDefaultKubeconfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Handle error appropriately, maybe return empty or log
		logger.Warning("Could not get user home directory: %v", err)
		return ""
	}
	return filepath.Join(homeDir, ".kube", "config")
}

// SetNamespaceForContext modifies the kubeconfig file to set the default namespace
// for a specific context AND sets that context as the current-context.
func SetNamespaceForContext(contextName, namespace string) error {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// Ensure we only load from the default path
	configPath := GetDefaultKubeconfigPath()
	if configPath == "" {
		return fmt.Errorf("could not determine default kubeconfig path")
	}
	loadingRules.ExplicitPath = configPath
	loadingRules.Precedence = []string{configPath} // Redundant with ExplicitPath but safe

	config, err := loadingRules.Load()
	if err != nil {
		logger.Error("Failed to load kubeconfig from '%s': %v", configPath, err)
		return fmt.Errorf("failed to load kubeconfig from '%s': %w", configPath, err)
	}

	context, exists := config.Contexts[contextName]
	if !exists {
		logger.Error("Context '%s' not found in kubeconfig '%s'", contextName, configPath)
		return fmt.Errorf("context '%s' not found in kubeconfig '%s'", contextName, configPath)
	}

	// Update the namespace for the specified context
	logger.Debug("Updating namespace for context '%s' to '%s' in kubeconfig '%s'", contextName, namespace, configPath)
	context.Namespace = namespace
	config.Contexts[contextName] = context // Update the map with the modified context

	// Set the specified context as the current context
	logger.Debug("Setting current-context to '%s' in kubeconfig '%s'", contextName, configPath)
	config.CurrentContext = contextName

	logger.Debug("Attempting to save updated kubeconfig to: %s (New current context: '%s')", configPath, config.CurrentContext)
	err = clientcmd.WriteToFile(*config, configPath)
	if err != nil {
		logger.Error("Failed to write updated kubeconfig to '%s': %v", configPath, err)
		return fmt.Errorf("failed to write updated kubeconfig to '%s': %w", configPath, err)
	}

	logger.Info("Successfully set namespace to '%s' and current-context to '%s' in kubeconfig '%s'", namespace, contextName, configPath)
	return nil
}
