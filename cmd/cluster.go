package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

var clusterCmd = &cobra.Command{
	Use: "cluster",
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(clusterCmd)
}
