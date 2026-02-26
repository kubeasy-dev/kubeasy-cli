/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/kubeasy-dev/kubeasy-cli/internal/constants"
	"github.com/kubeasy-dev/kubeasy-cli/internal/logger"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var noSpinner bool

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "kubeasy-cli",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize logger globally here with INFO level
		loggerOpts := logger.DefaultOptions()
		loggerOpts.FilePath = constants.LogFilePath
		loggerOpts.Level = logger.INFO

		// Attempt to truncate the log file at the start
		if err := os.Truncate(constants.LogFilePath, 0); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[WARN] Failed to clear log file %s: %v\n", constants.LogFilePath, err)
		}

		logger.Initialize(loggerOpts)
		logger.Info("Kubeasy CLI started - logging to: %s", constants.LogFilePath)

		// Enable CI mode if --no-spinner flag is set or stdout is not a TTY
		if noSpinner || !term.IsTerminal(int(os.Stdout.Fd())) {
			ui.SetCIMode(true)
		}
	},
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.kubeasy-cli.yaml)")

	rootCmd.PersistentFlags().BoolVar(&noSpinner, "no-spinner", false, "Force plain text output (spinners are disabled automatically when stdout is not a TTY)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
