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

Commands:
  create   - Scaffold a new challenge directory
  apply    - Deploy local challenge manifests to the Kind cluster
  validate - Run validations locally without submitting to API
  clean    - Remove dev challenge resources from the cluster`,
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
