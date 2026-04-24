package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Development tools for challenge creators",
	Long: `Development mode lets challenge creators scaffold, test, and iterate
on challenges locally without needing the Kubeasy API.

Dev commands operate on local files (Filesystem-first). They search for 
challenge.yaml in the current directory or ../challenges/<slug>/.

Commands:
  create   - Scaffold a new challenge directory
  get      - Display challenge metadata from local files
  apply    - Deploy challenge manifests from local files
  validate - Run validations locally without submitting to API
  test     - Apply manifests and run validations in one step
  clean    - Remove dev challenge resources from the cluster
  lint     - Validate challenge.yaml structure without a cluster
  status   - Show current challenge state (pods, events)
  logs     - Stream logs from challenge pods`,
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(devCmd)
}
