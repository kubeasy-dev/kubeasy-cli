/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/constants"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	debugLogging bool // Variable to store the debug flag value
)

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
		// Initialize logger globally here
		loggerOpts := logger.DefaultOptions()
		// Always set the file path
		loggerOpts.FilePath = constants.LogFilePath

		if debugLogging {
			// Attempt to truncate the log file at the start
			if err := os.Truncate(constants.LogFilePath, 0); err != nil && !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "[WARN] Failed to clear log file %s: %v\n", constants.LogFilePath, err)
			}
			loggerOpts.Level = logger.DEBUG
		} else {
			loggerOpts.Level = logger.INFO
		}

		logger.Initialize(loggerOpts)

		// Log the file path only when debugging, after initialization
		if debugLogging {
			logger.Debug("Logging debug messages to: %s (max lines: %d)",
				constants.LogFilePath, constants.MaxLogLines)
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
	// Add the persistent debug flag
	rootCmd.PersistentFlags().BoolVar(&debugLogging, "debug", false, "Enable debug logging to kubeasy-cli.log")

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.kubeasy-cli.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
