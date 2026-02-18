package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/kubeasy-dev/kubeasy-cli/internal/deployer"
	"github.com/kubeasy-dev/kubeasy-cli/internal/devutils"
	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/spf13/cobra"
)

var devApplyDir string

var devApplyCmd = &cobra.Command{
	Use:   "apply [challenge-slug]",
	Short: "Deploy local challenge manifests to the Kind cluster",
	Long: `Deploys challenge manifests from a local directory to the Kind cluster.
This is the dev equivalent of 'kubeasy challenge start' but reads from the local
filesystem instead of pulling from the OCI registry. No API login required.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		challengeSlug := args[0]

		ui.Section(fmt.Sprintf("Applying Dev Challenge: %s", challengeSlug))

		// Validate slug format
		if err := validateChallengeSlug(challengeSlug); err != nil {
			ui.Error("Invalid challenge slug")
			return err
		}

		// Resolve local challenge directory
		challengeDir, err := devutils.ResolveLocalChallengeDir(challengeSlug, devApplyDir)
		if err != nil {
			ui.Error("Failed to find challenge directory")
			return err
		}
		ui.Info(fmt.Sprintf("Using challenge directory: %s", challengeDir))

		// Get Kubernetes clients
		clientset, err := kube.GetKubernetesClient()
		if err != nil {
			ui.Error("Failed to get Kubernetes client. Is the cluster running? Try 'kubeasy setup'")
			return fmt.Errorf("failed to get Kubernetes client: %w", err)
		}

		dynamicClient, err := kube.GetDynamicClient()
		if err != nil {
			ui.Error("Failed to get dynamic client")
			return fmt.Errorf("failed to get dynamic client: %w", err)
		}

		// Build and load Docker image if image/ directory exists
		if deployer.HasImageDir(challengeDir) {
			imageDir := filepath.Join(challengeDir, "image")
			imageTag := challengeSlug + ":latest"
			ui.Info(fmt.Sprintf("Detected image/ directory, building '%s'...", imageTag))

			err = ui.TimedSpinner("Building and loading Docker image", func() error {
				return deployer.BuildAndLoadImage(cmd.Context(), imageDir, imageTag, constants.KubeasyClusterName)
			})
			if err != nil {
				ui.Error("Failed to build/load Docker image")
				return fmt.Errorf("failed to build/load Docker image: %w", err)
			}
		}

		// Create namespace
		err = ui.WaitMessage("Creating namespace", func() error {
			return kube.CreateNamespace(cmd.Context(), clientset, challengeSlug)
		})
		if err != nil {
			ui.Error("Failed to create namespace")
			return fmt.Errorf("failed to create namespace: %w", err)
		}

		// Deploy local manifests
		err = ui.TimedSpinner("Deploying challenge manifests", func() error {
			return deployer.DeployLocalChallenge(cmd.Context(), clientset, dynamicClient, challengeDir, challengeSlug)
		})
		if err != nil {
			ui.Error("Failed to deploy challenge")
			return fmt.Errorf("failed to deploy challenge: %w", err)
		}

		// Set kubectl context
		if err := kube.SetNamespaceForContext(constants.KubeasyClusterContext, challengeSlug); err != nil {
			ui.Warning(fmt.Sprintf("Failed to set kubectl context namespace: %v", err))
		}

		ui.Println()
		ui.Success(fmt.Sprintf("Challenge '%s' deployed from local files!", challengeSlug))
		ui.Info(fmt.Sprintf("Namespace: %s", challengeSlug))
		ui.Info("Run 'kubeasy dev validate " + challengeSlug + "' to test your validations")

		return nil
	},
}

func init() {
	devCmd.AddCommand(devApplyCmd)
	devApplyCmd.Flags().StringVar(&devApplyDir, "dir", "", "Path to challenge directory (default: auto-detect)")
}
