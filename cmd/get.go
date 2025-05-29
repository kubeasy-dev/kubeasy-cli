package cmd

import (
	"bufio"
	"fmt"
	"os"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/api"
	"github.com/spf13/cobra"
)

var getChallengeCmd = &cobra.Command{
	Use:   "get [challenge-slug]",
	Short: "Get and display details for a specific challenge",
	Long:  `Retrieves challenge details (description, content, etc.) from the Kubeasy API and displays them.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		challengeSlug := args[0]

		challenge, err := api.GetChallenge(challengeSlug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error retrieving challenge '%s': %v\n", challengeSlug, err)
			os.Exit(1)
		}
		if challenge == nil {
			fmt.Println("Challenge not found.")
			return
		}
		fmt.Printf("Challenge: %s\n", challenge.Title)
		fmt.Printf("Difficulty: %s   Theme: %s\n", challenge.Difficulty, challenge.Theme)
		fmt.Println()
		fmt.Println(challenge.Description)
		fmt.Println()
		fmt.Println("Initial Situation:")
		fmt.Println(challenge.InitialSituation)
		fmt.Println()
		fmt.Println("Objective:")
		fmt.Println(challenge.Objective)
		fmt.Println()
		fmt.Println("Press Enter to quit.")
		bufio.NewReader(os.Stdin).ReadBytes('\n')
	},
}

func init() {
	challengeCmd.AddCommand(getChallengeCmd)
}
