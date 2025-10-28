package cmd

import (
	"fmt"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/argocd"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/ui"
	"github.com/spf13/cobra"
	"sigs.k8s.io/kind/pkg/cluster"
)

func checkClusterExists() (bool, error) {
	provider := cluster.NewProvider()
	clusters, err := provider.List()
	if err != nil {
		ui.Error("Failed to list clusters")
		return false, fmt.Errorf("failed to list clusters: %w", err)
	}
	clusterExists := false
	for _, c := range clusters {
		if c == "kubeasy" {
			clusterExists = true
			break
		}
	}
	return clusterExists, nil
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup",
	Long:  "It will setup a local cluster for the Kubeasy challenges and install ArgoCD",
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.PrintLogo()
		ui.Section("Kubeasy Environment Setup")
		ui.Println()

		// Step 1: Check/Create cluster
		exists, err := checkClusterExists()
		if err != nil {
			return err
		}
		if !exists {
			err := ui.TimedSpinner("Creating kind cluster 'kubeasy'", func() error {
				return cluster.NewProvider().Create("kubeasy")
			})
			if err != nil {
				ui.Error("Failed to create kind cluster 'kubeasy'")
				return fmt.Errorf("failed to create kind cluster: %w", err)
			}
		} else {
			ui.Success("Kind cluster 'kubeasy' already exists")
		}

		// Step 2: Install ArgoCD
		isInstalled, err := argocd.IsArgoCDInstalled()
		if err != nil {
			ui.Error("Error checking ArgoCD installation status")
			return fmt.Errorf("failed to check ArgoCD installation: %w", err)
		}

		if isInstalled {
			ui.Success("ArgoCD is already installed")
			// Ensure default project and app-of-apps exist even if ArgoCD was already installed
			err := ui.TimedSpinner("Ensuring ArgoCD resources", argocd.EnsureArgoCDResources)
			if err != nil {
				ui.Error("Error ensuring ArgoCD resources")
				return fmt.Errorf("failed to ensure ArgoCD resources: %w", err)
			}
		} else {
			err := ui.TimedSpinner("Installing ArgoCD", func() error {
				options := argocd.DefaultInstallOptions()
				return argocd.InstallArgoCD(options)
			})
			if err != nil {
				ui.Error("Error installing ArgoCD")
				return fmt.Errorf("failed to install ArgoCD: %w", err)
			}
		}

		// Step 3: Wait for apps
		apps := []string{"kubeasy-cli-setup", "kyverno", "argocd", "kubeasy-challenge-operator"}
		err = ui.TimedSpinner("Waiting for ArgoCD applications to be ready", func() error {
			return argocd.WaitForArgoCDAppsReadyCore(apps, 8*time.Minute)
		})
		if err != nil {
			ui.Error("Error waiting for ArgoCD apps")
			return fmt.Errorf("failed to wait for ArgoCD apps: %w", err)
		}

		ui.Println()
		ui.Success("Kubeasy environment is ready!")
		ui.Info("You can now start challenges with 'kubeasy challenge start <slug>'")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
