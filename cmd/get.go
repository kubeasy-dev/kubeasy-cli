package cmd

import (
	"bufio"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var getChallengeCmd = &cobra.Command{
	Use:   "get [challenge-slug]",
	Short: "Get and display details for a specific challenge",
	Long:  `Retrieves challenge details (description, content, etc.) from the Kubeasy API and displays them.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		challengeSlug := args[0]

		challenge := getChallengeOrExit(challengeSlug)

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
		_, _ = bufio.NewReader(os.Stdin).ReadBytes('\n')
	},
}

func init() {
	challengeCmd.AddCommand(getChallengeCmd)
}
