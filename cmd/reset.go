package cmd

import (
	"fmt"
	"os"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/api"
	"github.com/spf13/cobra"
)

var resetChallengeCmd = &cobra.Command{
	Use:   "reset [challenge-slug]",
	Short: "Reset a challenge",
	Long:  `Resets a challenge by removing challenge namespace and resetting progress and submissions`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		challengeSlug := args[0]

		challenge, err := api.GetChallenge(challengeSlug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching challenge: %v\n", err)
			os.Exit(1)
		}

		if err := api.ResetChallengeProgress(challenge.Id); err != nil {
			fmt.Fprintf(os.Stderr, "Error resetting challenge '%s': %v\n", challengeSlug, err)
			os.Exit(1)
		}

		fmt.Printf("Challenge '%s' reset successfully.\n", challengeSlug)
	},
}

func init() {
	challengeCmd.AddCommand(resetChallengeCmd)
}
