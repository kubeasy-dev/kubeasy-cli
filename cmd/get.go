package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/api"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/logger"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/ui"
	"github.com/spf13/cobra"
)

var getChallengeCmd = &cobra.Command{
	Use:   "get [challenge-slug]",
	Short: "Get and display details for a specific challenge",
	Long:  `Retrieves challenge details (description, content, etc.) from the Kubeasy API and displays them.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		challengeSlug := args[0]
		logger.Info("Attempting to get challenge: %s", challengeSlug)

		// Initialize the model
		m := ui.NewChallengeModel()
		p := tea.NewProgram(m, tea.WithAltScreen()) // Use AltScreen for better UI separation

		// Retrieve the challenge asynchronously
		go func() {
			logger.Debug("Goroutine started to fetch challenge '%s' from API", challengeSlug)
			challenge, err := api.GetChallenge(challengeSlug)
			if err != nil {
				logger.Error("Failed to get challenge '%s': %v", challengeSlug, err)
				p.Send(fmt.Errorf("failed to retrieve challenge '%s': %w", challengeSlug, err)) // Send wrapped error
				return
			}
			logger.Debug("Successfully fetched challenge '%s', sending to UI model", challengeSlug)
			p.Send(challenge) // Send the challenge data
		}()

		// Run the Bubbletea program
		finalModel, err := p.Run()
		if err != nil {
			// Log the error from Bubble Tea itself
			logger.Error("Bubble Tea program finished with error: %v", err)
			fmt.Fprintf(os.Stderr, "Error displaying challenge: %v\n", err)
			os.Exit(1)
		}

		// Check if the final model contains an error that was handled internally by Bubble Tea
		if finalChallengeModel, ok := finalModel.(ui.ChallengeModel); ok && finalChallengeModel.Error != nil {
			// The error was already logged when received by the model, just exit non-zero
			// The error message was displayed by the Bubble Tea View
			os.Exit(1)
		} else {
			logger.Info("Successfully displayed challenge: %s", challengeSlug)
		}
	},
}

func init() {
	challengeCmd.AddCommand(getChallengeCmd)
}
