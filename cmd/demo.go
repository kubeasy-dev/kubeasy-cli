package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

var demoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Try Kubeasy without an account",
	Long: `Demo mode lets you try Kubeasy without signing up.

Get a demo token at https://kubeasy.dev/get-started and use it to:
1. Set up a local cluster
2. Create an nginx pod
3. Submit your solution`,
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(demoCmd)
}
