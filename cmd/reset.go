package cmd

import (
	"fmt"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/api"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/ui"
	"github.com/spf13/cobra"
)

var resetChallengeCmd = &cobra.Command{
	Use:   "reset [challenge-slug]",
	Short: "Reset a challenge",
	Long:  `Resets a challenge by removing challenge namespace and resetting progress and submissions`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		challengeSlug := args[0]

		ui.Section(fmt.Sprintf("Resetting Challenge: %s", challengeSlug))

		// Verify challenge exists
		_, err := getChallenge(challengeSlug)
		if err != nil {
			ui.Error(err.Error())
			return err
		}

		// Delete resources
		if err := deleteChallengeResources(cmd.Context(), challengeSlug); err != nil {
			return err
		}

		// Reset progress on server
		err = ui.WaitMessage("Resetting challenge progress on server", func() error {
			return api.ResetChallengeProgress(challengeSlug)
		})
		if err != nil {
			ui.Error("Failed to reset challenge progress")
			return fmt.Errorf("failed to reset challenge progress: %w", err)
		}

		ui.Println()
		ui.Success(fmt.Sprintf("Challenge '%s' reset successfully!", challengeSlug))
		ui.Info("You can start the challenge again with 'kubeasy challenge start " + challengeSlug + "'")

		return nil
	},
}

func init() {
	challengeCmd.AddCommand(resetChallengeCmd)
}
