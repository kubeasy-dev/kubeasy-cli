package cmd

import (
	"bufio"
	"os"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/ui"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var getChallengeCmd = &cobra.Command{
	Use:   "get [challenge-slug]",
	Short: "Get and display details for a specific challenge",
	Long:  `Retrieves challenge details (description, content, etc.) from the Kubeasy API and displays them.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		challengeSlug := args[0]

		challenge, err := getChallenge(challengeSlug)
		if err != nil {
			ui.Error(err.Error())
			return err
		}

		ui.Println()
		ui.Section(challenge.Title)

		// Display metadata
		ui.KeyValue("Difficulty", challenge.Difficulty)
		ui.KeyValue("Theme", challenge.Theme)
		ui.KeyValue("Slug", challenge.Slug)

		ui.Println()

		// Display description in a panel
		if challenge.Description != "" {
			ui.Panel("Description", challenge.Description)
			ui.Println()
		}

		// Display initial situation
		if challenge.InitialSituation != "" {
			pterm.DefaultSection.Println("Initial Situation")
			pterm.Println(challenge.InitialSituation)
			ui.Println()
		}

		// Display objective
		if challenge.Objective != "" {
			pterm.DefaultSection.Println("Objective")
			pterm.Println(challenge.Objective)
			ui.Println()
		}

		ui.Info("Press Enter to continue...")
		_, _ = bufio.NewReader(os.Stdin).ReadBytes('\n')

		return nil
	},
}

func init() {
	challengeCmd.AddCommand(getChallengeCmd)
}
