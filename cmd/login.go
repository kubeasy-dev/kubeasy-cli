package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/api"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/constants"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/ui"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
	"golang.org/x/term"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to Kubeasy by storing your API key",
	Long: `Login to Kubeasy by securely storing your API key in the system keychain.

This command will prompt you for your API key.
If you don't have an API key or forgot it, visit https://kubeasy.dev/profile

After successful login, you will be able to use commands requiring authentication.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Section("Login to Kubeasy")

		// Check if token already exists
		existingToken, err := keyring.Get(constants.KeyringServiceName, "api_key")
		if err == nil && strings.TrimSpace(existingToken) != "" {
			// Build expiration info from JWT
			expInfo := ""
			if token, _, p := new(jwt.Parser).ParseUnverified(existingToken, jwt.MapClaims{}); p == nil {
				if claims, ok := token.Claims.(jwt.MapClaims); ok {
					if v, ok := claims["exp"].(float64); ok && v > 0 {
						expiresAt := time.Unix(int64(v), 0)
						if expiresAt.After(time.Unix(0, 0)) {
							expInfo = fmt.Sprintf(" (expires %s)", expiresAt.Local().Format(time.RFC1123))
						}
					}
				}
			}

			ui.Info(fmt.Sprintf("An API token is already saved%s", expInfo))
			reuse := ui.Confirmation("Do you want to reuse it?")

			if reuse {
				// Try to fetch and display profile
				profile, perr := api.GetProfile()
				if perr != nil {
					ui.Warning("Token exists but failed to fetch profile")
					ui.Info("You may need to login again")
				} else {
					fullName := strings.TrimSpace(profile.FirstName + " " + profile.LastName)
					if fullName != "" {
						ui.KeyValue("Profile", fullName)
					}
					ui.Success("Already logged in!")
					return nil
				}
			}
		}

		// Fresh login flow
		ui.Println()
		ui.Info("Please enter your API key to login")
		ui.Info("Get your API key at: https://kubeasy.dev/profile")
		ui.Println()
		fmt.Print("API Key: ")

		// Read the API key without echoing input
		byteKey, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()

		if err != nil {
			ui.Error("Failed to read API key")
			return nil
		}

		apiKey := strings.TrimSpace(string(byteKey))
		if apiKey == "" {
			ui.Error("API key cannot be empty")
			return nil
		}

		// Store the key
		err = keyring.Set(constants.KeyringServiceName, "api_key", apiKey)
		if err != nil {
			ui.Error("Failed to store API key in keyring")
			return nil
		}

		ui.Success("API key stored successfully")

		// Verify by fetching profile
		profile, err := api.GetProfile()
		if err != nil {
			ui.Warning("Logged in, but failed to fetch profile")
		} else {
			fullName := strings.TrimSpace(profile.FirstName + " " + profile.LastName)
			if fullName != "" {
				ui.KeyValue("Welcome", fullName)
			}
		}

		ui.Println()
		ui.Success("Login successful!")
		ui.Info("You can now use Kubeasy commands")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
