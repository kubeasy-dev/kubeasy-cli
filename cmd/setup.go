package cmd

import (
	"fmt"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/api"
	"github.com/kubeasy-dev/kubeasy-cli/internal/argocd"
	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/kubeasy-dev/kubeasy-cli/internal/keystore"
	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
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

		// Require authentication
		if token, err := keystore.Get(); err != nil || token == "" {
			ui.Error("You must be logged in to set up Kubeasy")
			ui.Info("Run 'kubeasy login' first")
			return nil
		}

		// Step 1: Check/Create cluster
		exists, err := checkClusterExists()
		if err != nil {
			return err
		}
		if !exists {
			err := ui.TimedSpinner(fmt.Sprintf("Creating kind cluster 'kubeasy' (Kubernetes %s)", constants.GetKubernetesVersion()), func() error {
				return cluster.NewProvider().Create(
					"kubeasy",
					cluster.CreateWithNodeImage(constants.KindNodeImage),
				)
			})
			if err != nil {
				ui.Error(fmt.Sprintf("Failed to create Kind cluster with image %s", constants.KindNodeImage))
				ui.Info("Verify that the Kind node image is available")
				ui.Info("You can manually pull: docker pull " + constants.KindNodeImage)
				return fmt.Errorf("failed to create kind cluster with image %s: %w", constants.KindNodeImage, err)
			}
		} else {
			// Detect actual cluster version and compare with expected
			actualVersion, err := kube.GetServerVersion()
			if err != nil {
				// Log at debug level for troubleshooting, but don't block setup
				logger.Debug("Could not detect cluster version: %v", err)
				ui.Success("Kind cluster 'kubeasy' already exists")
				ui.Info("Could not verify cluster version - cluster may need configuration")
			} else {
				expectedVersion := constants.GetKubernetesVersion()
				// Compare major.minor versions to handle build metadata (+k3s1, -eks) and patch differences
				if !constants.VersionsCompatible(actualVersion, expectedVersion) {
					actualMajorMinor := constants.GetMajorMinorVersion(actualVersion)
					expectedMajorMinor := constants.GetMajorMinorVersion(expectedVersion)
					ui.Warning(fmt.Sprintf("Kind cluster 'kubeasy' exists with Kubernetes %s (expected %s)", actualMajorMinor, expectedMajorMinor))
					ui.Info("Consider recreating: kind delete cluster -n kubeasy && kubeasy setup")
				} else {
					ui.Success(fmt.Sprintf("Kind cluster 'kubeasy' already exists (Kubernetes %s)", constants.GetMajorMinorVersion(actualVersion)))
				}
			}
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

		// Step 3: Wait for apps (kyverno, argocd, and local-path-provisioner are applied directly from embedded manifests)
		apps := []string{"kyverno", "argocd", "local-path-provisioner"}
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

		api.TrackEvent("/track/setup")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
