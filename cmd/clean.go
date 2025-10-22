package cmd

import (
	"fmt"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/ui"
	"github.com/spf13/cobra"
)

var cleanChallengeCmd = &cobra.Command{
	Use:   "clean [challenge-slug]",
	Short: "Clean a challenge",
	Long:  `Cleans a challenge by removing challenge all associated resources`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		challengeSlug := args[0]

		ui.Section(fmt.Sprintf("Cleaning Challenge: %s", challengeSlug))

		if err := deleteChallengeResources(cmd.Context(), challengeSlug); err != nil {
			return err
		}

		ui.Println()
		ui.Success(fmt.Sprintf("Challenge '%s' cleaned successfully!", challengeSlug))
		ui.Info("All resources have been removed from your cluster")

		return nil
	},
}

func init() {
	challengeCmd.AddCommand(cleanChallengeCmd)
}
