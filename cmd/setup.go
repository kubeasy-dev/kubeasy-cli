package cmd

import (
	"fmt"

	"github.com/kubeasy-dev/kubeasy-cli/internal/api"
	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/kubeasy-dev/kubeasy-cli/internal/deployer"
	"github.com/kubeasy-dev/kubeasy-cli/internal/keystore"
	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/spf13/cobra"
	kindv1alpha4 "sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
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

// kindClusterConfig returns the Kind cluster configuration with extraPortMappings for nginx-ingress.
func kindClusterConfig() *kindv1alpha4.Cluster {
	return &kindv1alpha4.Cluster{
		TypeMeta: kindv1alpha4.TypeMeta{
			Kind:       "Cluster",
			APIVersion: "kind.x-k8s.io/v1alpha4",
		},
		Nodes: []kindv1alpha4.Node{
			{
				Role: kindv1alpha4.ControlPlaneRole,
				ExtraPortMappings: []kindv1alpha4.PortMapping{
					{ContainerPort: 80, HostPort: 8080, Protocol: kindv1alpha4.PortMappingProtocolTCP},
					{ContainerPort: 443, HostPort: 8443, Protocol: kindv1alpha4.PortMappingProtocolTCP},
				},
			},
		},
	}
}

// createClusterWithConfig writes the Kind config and creates the cluster with port mappings.
func createClusterWithConfig() error {
	cfg := kindClusterConfig()

	// Write config before creating cluster so it is available for future checks.
	if err := deployer.WriteKindConfig(cfg); err != nil {
		logger.Debug("Could not write kind config: %v", err)
		// Non-fatal — continue with cluster creation.
	}

	return cluster.NewProvider().Create(
		"kubeasy",
		cluster.CreateWithV1Alpha4Config(cfg),
		cluster.CreateWithNodeImage(constants.KindNodeImage),
	)
}

// printComponentResult prints a single component status line to stdout.
func printComponentResult(r deployer.ComponentResult) {
	switch r.Status {
	case deployer.StatusReady:
		ui.Success(fmt.Sprintf("%s: ready", r.Name))
	case deployer.StatusNotReady:
		ui.Error(fmt.Sprintf("%s: not-ready — %s", r.Name, r.Message))
	case deployer.StatusMissing:
		ui.Warning(fmt.Sprintf("%s: missing — %s", r.Name, r.Message))
	}
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup",
	Long:  "It will setup a local cluster for the Kubeasy challenges and install infrastructure components",
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.PrintLogo()
		ui.Section("Kubeasy Environment Setup")
		ui.Println()

		// Require authentication
		if token, err := keystore.Get(); err != nil || token == "" {
			ui.Error("You must be logged in to set up Kubeasy")
			ui.Info("Run 'kubeasy login' first")
			return fmt.Errorf("authentication required: run 'kubeasy login' first")
		}

		// Step 1: Check/Create cluster
		exists, err := checkClusterExists()
		if err != nil {
			return err
		}

		ref := kindClusterConfig()

		if !exists {
			// Cluster does not exist — create with port mappings.
			err := ui.TimedSpinner(
				fmt.Sprintf("Creating kind cluster 'kubeasy' (Kubernetes %s)", constants.GetKubernetesVersion()),
				createClusterWithConfig,
			)
			if err != nil {
				ui.Error(fmt.Sprintf("Failed to create Kind cluster with image %s", constants.KindNodeImage))
				ui.Info("Verify that the Kind node image is available")
				ui.Info("You can manually pull: docker pull " + constants.KindNodeImage)
				return fmt.Errorf("failed to create kind cluster with image %s: %w", constants.KindNodeImage, err)
			}
		} else {
			// Cluster already exists — compare installed config against reference.
			if !deployer.KindConfigMatches(ref) {
				ui.Warning("Kind cluster 'kubeasy' configuration has drifted from the current reference")
				ui.Info("The installed cluster was created with a different configuration (e.g. missing port mappings or updated settings)")
				confirmed := ui.Confirmation("Recreate cluster with the updated configuration? (This will DELETE the existing cluster)")
				if confirmed {
					err := ui.TimedSpinner("Deleting existing cluster...", func() error {
						return cluster.NewProvider().Delete("kubeasy", "")
					})
					if err != nil {
						ui.Error("Failed to delete existing cluster")
						return fmt.Errorf("failed to delete kind cluster: %w", err)
					}
					err = ui.TimedSpinner(
						fmt.Sprintf("Recreating kind cluster 'kubeasy' (Kubernetes %s)", constants.GetKubernetesVersion()),
						createClusterWithConfig,
					)
					if err != nil {
						ui.Error(fmt.Sprintf("Failed to recreate Kind cluster with image %s", constants.KindNodeImage))
						ui.Info("Verify that the Kind node image is available")
						ui.Info("You can manually pull: docker pull " + constants.KindNodeImage)
						return fmt.Errorf("failed to recreate kind cluster with image %s: %w", constants.KindNodeImage, err)
					}
				} else {
					ui.Warning("Skipping cluster recreation — some features may not work correctly")
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
		}

		// Step 2: Install all infrastructure components with per-component status output.
		ui.Section("Installing Components")

		clientset, err := kube.GetKubernetesClient()
		if err != nil {
			ui.Error("Failed to get Kubernetes client")
			return fmt.Errorf("failed to get Kubernetes client: %w", err)
		}
		dynamicClient, err := kube.GetDynamicClient()
		if err != nil {
			ui.Error("Failed to get Kubernetes dynamic client")
			return fmt.Errorf("failed to get Kubernetes dynamic client: %w", err)
		}

		results := deployer.SetupAllComponents(cmd.Context(), clientset, dynamicClient)
		allReady := true
		for _, r := range results {
			printComponentResult(r)
			if r.Status != deployer.StatusReady {
				allReady = false
			}
		}

		ui.Println()

		if !allReady {
			ui.Error("Some components failed to install. Run 'kubeasy setup' again or check logs with --debug.")
			return fmt.Errorf("setup incomplete: one or more components failed")
		}

		ui.Success("Kubeasy environment is ready!")
		ui.Info("You can now start challenges with 'kubeasy challenge start <slug>'")

		api.TrackSetup(cmd.Context())

		return nil
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
