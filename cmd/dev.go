package cmd

import (
	"log"
	"os"

	"github.com/spf13/cobra"
)

// devRegistryURL is the registry URL used by all dev subcommands.
// Defaults to the local registry; override with --registry or KUBEASY_REGISTRY_URL.
var devRegistryURL string

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Development tools for challenge creators",
	Long: `Development mode lets challenge creators scaffold, test, and iterate
on challenges locally without needing the Kubeasy API.

By default, dev commands connect to a local registry at http://localhost:8080.
Start it with: go run ./cmd/serve (inside the registry repo).
Override with --registry or KUBEASY_REGISTRY_URL.

Commands:
  create   - Scaffold a new challenge directory
  get      - Display challenge metadata from the local registry
  apply    - Deploy challenge manifests from the local registry
  validate - Run validations locally without submitting to API
  test     - Apply manifests and run validations in one step
  clean    - Remove dev challenge resources from the cluster
  lint     - Validate challenge.yaml structure without a cluster
  status   - Show current challenge state (pods, events)
  logs     - Stream logs from challenge pods`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Allow env var override
		if envURL := os.Getenv("KUBEASY_REGISTRY_URL"); envURL != "" && !cmd.Flags().Changed("registry") {
			devRegistryURL = envURL
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(devCmd)
	devCmd.PersistentFlags().StringVar(&devRegistryURL, "registry", "http://localhost:8080", "Local registry URL (default: http://localhost:8080)")
}
