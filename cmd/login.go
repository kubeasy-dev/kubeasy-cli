package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/constants"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/logger" // Import logger
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to Kubeasy by storing your API key",
	Long: `Login to Kubeasy by securely storing your API key in the system keychain.

This command will prompt you for your API key.
If you don't have an API key or forgot it, visit https://kubeasy.dev/profile

After successful login, you will be able to use commands requiring authentication.`, // Updated description
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("Starting login process...")

		// Use huh to create a secure input field
		var apiKey string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Kubeasy API Key").
					EchoMode(huh.EchoModePassword).
					Placeholder("Enter your API key").
					Description("If you don't have an API key or forgot it, visit https://kubeasy.dev/profile").
					Value(&apiKey).
					// Add validation to ensure the key is not empty
					Validate(func(s string) error {
						if s == "" {
							return errors.New("API key cannot be empty.")
						}
						return nil
					}),
			),
		)

		// Run the input form
		logger.Debug("Running API key input form...")
		if err := form.Run(); err != nil {
			// Check for specific huh errors like cancellation
			if errors.Is(err, huh.ErrUserAborted) {
				logger.Warning("Login aborted by user.")
				fmt.Println("Login cancelled.")
			} else {
				logger.Error("Error during API key input: %v", err)
				fmt.Fprintf(os.Stderr, "Error during input: %v\n", err)
			}
			os.Exit(1)
		}
		logger.Debug("API key input successful.")

		// Store the API key directly
		logger.Info("Storing API key in system keychain...")
		err := keyring.Set(constants.KeyringServiceName, "api_key", apiKey)
		if err != nil {
			logger.Error("Error storing API key in keychain: %v", err)
			fmt.Fprintf(os.Stderr, "Error storing API key: %v\n", err)
			os.Exit(1)
		}

		logger.Info("API key successfully stored.")
		fmt.Println("✅ API key successfully stored!")
		fmt.Println("You can now use Kubeasy commands requiring authentication.")
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
