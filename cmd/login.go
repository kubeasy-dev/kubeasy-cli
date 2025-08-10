package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/api"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/constants"
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
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🔐 Login to Kubeasy")

		// 1) If a token already exists, propose to reuse it (show expiration info if available)
		existingToken, err := keyring.Get(constants.KeyringServiceName, "api_key")
		if err == nil && strings.TrimSpace(existingToken) != "" {
			// Build optional expiration info from JWT without verifying signature
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
			fmt.Printf("An API token is already saved%s. Reuse it? [Y/n]: ", expInfo)
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer == "" || answer == "y" || answer == "yes" || answer == "o" || answer == "oui" {
				// Try to fetch and display profile to confirm
				profile, perr := api.GetProfile()
				if perr != nil {
					fmt.Printf("ℹ️ Reused token, but failed to fetch profile: %v\n", perr)
				} else {
					fullName := strings.TrimSpace(profile.FirstName + " " + profile.LastName)
					if fullName == "" {
						fmt.Println("👤 Profile fetched successfully.")
					} else {
						fmt.Printf("👤 Profile: %s\n", fullName)
					}
				}
				fmt.Println("You can now use Kubeasy commands.")
				return
			}
			// User chose fresh login; continue below
		}

		// 2) Fresh login flow
		fmt.Println("Please enter your API key to login.")
		fmt.Println("If you don't have an API key or forgot it, please visit https://kubeasy.dev/profile")
		fmt.Print("API Key: ")
		// Read the API key without echoing input
		byteKey, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			fmt.Printf("❌ Error reading API key: %v\n", err)
			return
		}
		apiKey := strings.TrimSpace(string(byteKey))
		if apiKey == "" {
			fmt.Println("❌ API key cannot be empty.")
			return
		}

		err = keyring.Set(constants.KeyringServiceName, "api_key", apiKey)
		if err != nil {
			fmt.Printf("❌ Error storing API key: %v\n", err)
			return
		}
		fmt.Println("✅ API key successfully stored!")
		// Fetch and display user profile to confirm the token works
		profile, err := api.GetProfile()
		if err != nil {
			fmt.Printf("ℹ️ Logged in, but failed to fetch profile: %v\n", err)
		} else {
			fullName := strings.TrimSpace(profile.FirstName + " " + profile.LastName)
			if fullName == "" {
				fmt.Println("👤 Profile fetched successfully.")
			} else {
				fmt.Printf("👤 Profile: %s\n", fullName)
			}
		}
		fmt.Println("You can now use Kubeasy commands.")
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
