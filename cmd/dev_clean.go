package cmd

import (
	"fmt"

	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/spf13/cobra"
)

var devCleanCmd = &cobra.Command{
	Use:   "clean [challenge-slug]",
	Short: "Remove dev challenge resources from the cluster",
	Long: `Removes all Kubernetes resources for a dev challenge (deletes the namespace).
This is the dev equivalent of 'kubeasy challenge clean' but does not require
API login.`,
	Args: cobra.ExactArgs(1),
	Args:          cobra.ExactArgs(1),
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		challengeSlug := args[0]

		ui.Section(fmt.Sprintf("Cleaning Dev Challenge: %s", challengeSlug))

		// Validate slug format
		if err := validateChallengeSlug(challengeSlug); err != nil {
			ui.Error("Invalid challenge slug")
			return err
		}

		if err := deleteChallengeResources(cmd.Context(), challengeSlug); err != nil {
			return err
		}

		ui.Println()
		ui.Success(fmt.Sprintf("Dev challenge '%s' cleaned successfully!", challengeSlug))
		ui.Info("All resources have been removed from your cluster")

		return nil
	},
}

func init() {
	devCmd.AddCommand(devCleanCmd)
}
