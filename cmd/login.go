package cmd

import (
	"fmt"
	"os"
	"strings"

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
		fmt.Println("You can now use Kubeasy commands.")
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
