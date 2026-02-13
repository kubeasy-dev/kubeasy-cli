package cmd

import (
	"fmt"

	"github.com/kubeasy-dev/kubeasy-cli/internal/api"
	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/kubeasy-dev/kubeasy-cli/internal/deployer"
	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/spf13/cobra"
)

var startChallengeCmd = &cobra.Command{
	Use:   "start [challenge-slug]",
	Short: "Start a challenge",
	Long:  `Starts a challenge by installing the necessary components into the local Kubernetes cluster.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		challengeSlug := args[0]

		ui.Section(fmt.Sprintf("Starting Challenge: %s", challengeSlug))

		// Fetch challenge details
		var challenge *api.ChallengeEntity
		err := ui.WaitMessage("Fetching challenge details", func() error {
			var err error
			challenge, err = api.GetChallenge(challengeSlug)
			return err
		})
		if err != nil {
			ui.Error("Failed to fetch challenge")
			return fmt.Errorf("failed to fetch challenge: %w", err)
		}

		ui.Info(fmt.Sprintf("Challenge: %s", challenge.Title))

		// Check progress
		var progress *api.ChallengeStatusResponse
		err = ui.WaitMessage("Checking challenge progress", func() error {
			var err error
			progress, err = api.GetChallengeProgress(challengeSlug)
			return err
		})
		if err != nil {
			ui.Error("Failed to fetch challenge progress")
			return fmt.Errorf("failed to fetch challenge progress: %w", err)
		}

		if progress != nil && (progress.Status == "in_progress" || progress.Status == "completed") {
			ui.Warning("Challenge already started")
			ui.Info(fmt.Sprintf("Continue the challenge or reset it with 'kubeasy challenge reset %s'", challengeSlug))
			return nil // Not an error, just already started
		}

		// Setup environment - use context from command
		ctx := cmd.Context()
		ui.Println()

		// Step 1: Create namespace
		dynamicClient, err := kube.GetDynamicClient()
		if err != nil {
			ui.Error("Failed to get Kubernetes dynamic client")
			return fmt.Errorf("failed to get dynamic client: %w", err)
		}

		staticClient, err := kube.GetKubernetesClient()
		if err != nil {
			ui.Error("Failed to get Kubernetes static client")
			return fmt.Errorf("failed to get static client: %w", err)
		}

		err = ui.WaitMessage("Creating namespace", func() error {
			return kube.CreateNamespace(ctx, staticClient, challengeSlug)
		})
		if err != nil {
			ui.Error("Failed to create namespace")
			return fmt.Errorf("failed to create namespace: %w", err)
		}

		// Step 2: Deploy challenge via OCI
		err = ui.WaitMessage("Deploying challenge", func() error {
			return deployer.DeployChallenge(ctx, staticClient, dynamicClient, challengeSlug)
		})
		if err != nil {
			ui.Error("Failed to deploy challenge")
			return fmt.Errorf("failed to deploy challenge: %w", err)
		}

		// Step 3: Configure context
		if err := kube.SetNamespaceForContext(constants.KubeasyClusterContext, challengeSlug); err != nil {
			logger.Debug("Failed to set namespace for context: %v", err)
			ui.Warning("Could not configure kubectl context namespace")
		} else {
			ui.Success("Kubectl context configured")
		}

		// Step 4: Register progress
		err = ui.WaitMessage("Registering challenge progress", func() error {
			return api.StartChallenge(challengeSlug)
		})
		if err != nil {
			ui.Error("Failed to start challenge")
			return fmt.Errorf("failed to start challenge: %w", err)
		}

		ui.Println()
		ui.Success("Challenge environment is ready!")
		ui.KeyValue("Challenge", challengeSlug)
		ui.KeyValue("Namespace", challengeSlug)
		ui.KeyValue("Context", "kind-kubeasy")
		ui.Println()
		ui.Info("You can now start working on the challenge!")
		return nil
	},
}

func init() {
	challengeCmd.AddCommand(startChallengeCmd)
}
