package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"sigs.k8s.io/kind/pkg/cluster"

	"github.com/kubeasy-dev/kubeasy-cli/internal/api"
	"github.com/kubeasy-dev/kubeasy-cli/internal/argocd"
	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/kubeasy-dev/kubeasy-cli/internal/demo"
	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
)

var demoToken string

var demoStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the demo challenge",
	Long: `Start the demo challenge with your token from https://kubeasy.dev/get-started

This command will:
1. Verify your demo token
2. Set up a local Kind cluster (if not already set up)
3. Create a 'demo' namespace
4. Store your token locally for submission`,
	Example: "kubeasy demo start --token=abc123xyz789ab",
	RunE:    runDemoStart,
}

func init() {
	demoCmd.AddCommand(demoStartCmd)
	demoStartCmd.Flags().StringVar(&demoToken, "token", "", "Demo token from https://kubeasy.dev/get-started (required)")
	_ = demoStartCmd.MarkFlagRequired("token")
}

func runDemoStart(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	ui.PrintLogo()
	ui.Section("Starting Demo Mode")

	// 1. Verify token format (16 chars alphanumeric)
	if len(demoToken) != 16 {
		ui.Error("Invalid token format. Get a valid token at https://kubeasy.dev/get-started")
		return fmt.Errorf("invalid token format")
	}

	// 2. Verify token with backend
	ui.Info("Verifying demo token...")
	session, err := api.VerifyDemoToken(demoToken)
	if err != nil {
		ui.Error("Failed to verify token: " + err.Error())
		return err
	}

	if !session.Valid {
		ui.Error("Token is invalid or expired. Get a new one at https://kubeasy.dev/get-started")
		return fmt.Errorf("invalid token")
	}

	if session.CompletedAt > 0 {
		ui.Warning("This demo has already been completed!")
		ui.Info("You can still practice, but your results won't be tracked.")
	}

	ui.Success("Token verified!")

	// 3. Check if cluster exists, run setup if not
	ui.Info("Checking cluster status...")

	exists, err := checkClusterExists()
	if err != nil {
		return err
	}

	if !exists {
		// Create the cluster
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

		// Install ArgoCD
		err = ui.TimedSpinner("Installing ArgoCD", func() error {
			options := argocd.DefaultInstallOptions()
			return argocd.InstallArgoCD(options)
		})
		if err != nil {
			ui.Error("Error installing ArgoCD")
			return fmt.Errorf("failed to install ArgoCD: %w", err)
		}

		// Wait for ArgoCD apps
		apps := []string{"kyverno", "argocd", "local-path-provisioner"}
		err = ui.TimedSpinner("Waiting for ArgoCD applications to be ready", func() error {
			return argocd.WaitForArgoCDAppsReadyCore(apps, 8*time.Minute)
		})
		if err != nil {
			ui.Error("Error waiting for ArgoCD apps")
			return fmt.Errorf("failed to wait for ArgoCD apps: %w", err)
		}
	} else {
		ui.Success("Cluster is ready!")

		// Ensure ArgoCD is installed
		isInstalled, err := argocd.IsArgoCDInstalled()
		if err != nil {
			ui.Error("Error checking ArgoCD installation status")
			return fmt.Errorf("failed to check ArgoCD installation: %w", err)
		}

		if !isInstalled {
			err = ui.TimedSpinner("Installing ArgoCD", func() error {
				options := argocd.DefaultInstallOptions()
				return argocd.InstallArgoCD(options)
			})
			if err != nil {
				ui.Error("Error installing ArgoCD")
				return fmt.Errorf("failed to install ArgoCD: %w", err)
			}

			apps := []string{"kyverno", "argocd", "local-path-provisioner"}
			err = ui.TimedSpinner("Waiting for ArgoCD applications to be ready", func() error {
				return argocd.WaitForArgoCDAppsReadyCore(apps, 8*time.Minute)
			})
			if err != nil {
				ui.Error("Error waiting for ArgoCD apps")
				return fmt.Errorf("failed to wait for ArgoCD apps: %w", err)
			}
		}
	}

	// 4. Create demo namespace
	ui.Info("Creating demo namespace...")

	clientset, err := kube.GetKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to get kubernetes client: %w", err)
	}

	if err := kube.CreateNamespace(ctx, clientset, demo.DemoNamespace); err != nil {
		// Namespace might already exist, that's OK
		logger.Debug("Namespace creation: %v", err)
	}

	ui.Success("Namespace 'demo' is ready!")

	// 5. Store token locally
	if err := demo.SaveToken(demoToken); err != nil {
		ui.Warning("Could not save token locally: " + err.Error())
		ui.Info("You'll need to provide the token again when submitting.")
	}

	// 6. Set kubectl context to demo namespace
	if err := kube.SetNamespaceForContext("kind-kubeasy", demo.DemoNamespace); err != nil {
		logger.Warning("Could not set namespace context: %v", err)
	}

	// 7. Notify backend that demo started (triggers realtime update on frontend)
	if err := api.SendDemoStart(demoToken); err != nil {
		logger.Debug("Failed to notify demo start: %v", err)
	}

	return nil
}
