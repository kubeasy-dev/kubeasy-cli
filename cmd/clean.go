package cmd

import (

	"fmt"






	"github.com/spf13/cobra"
)

var cleanChallengeCmd = &cobra.Command{
	Use:   "clean [challenge-slug]",
	Short: "Clean a challenge",
	Long:  `Cleans a challenge by removing challenge all associated resources`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		challengeSlug := args[0]

		deleteChallengeResources(challengeSlug)


		fmt.Printf("Challenge '%s' cleaned successfully.\n", challengeSlug)
	},
}

func init() {
	challengeCmd.AddCommand(cleanChallengeCmd)
}
