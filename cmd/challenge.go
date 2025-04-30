package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

var challengeCmd = &cobra.Command{
	Use: "challenge",
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(challengeCmd)
}
