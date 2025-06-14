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

		deleteChallengeResources(challengeSlug)
		challenge := getChallengeOrExit(challengeSlug)

		// Reset challenge progress
		if err := api.ResetChallengeProgress(challenge.ID); err != nil {
			fmt.Fprintf(os.Stderr, "Error resetting challenge '%s': %v\n", challengeSlug, err)
			os.Exit(1)
		}

		fmt.Printf("Challenge '%s' reset successfully (including ArgoCD app and subresources).\n", challengeSlug)
	},
}

func init() {
	challengeCmd.AddCommand(resetChallengeCmd)
}
