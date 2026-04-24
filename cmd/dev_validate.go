package cmd

import (
	"fmt"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/devutils"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	devValidateDir           string
	devValidateWatch         bool
	devValidateWatchInterval time.Duration
	devValidateFailFast      bool
	devValidateJSON          bool
)

var devValidateCmd = &cobra.Command{
	Use:   "validate [challenge-slug]",
	Short: "Run validations locally without submitting to API",
	Long: `Runs challenge validations against the Kind cluster and displays results.
This is the dev equivalent of 'kubeasy challenge submit' but does not send
results to the Kubeasy API. No login required.

It searches for challenge.yaml in the current directory or ../challenges/<slug>/.
Use --dir to specify a custom directory.
Use --watch to continuously re-run validations at the given interval.
Use --fail-fast to stop at the first validation failure.
Use --json for structured JSON output (useful for CI).`,
	Args:          cobra.ExactArgs(1),
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		challengeSlug := args[0]

		opts := DevValidateOpts{
			FailFast:   devValidateFailFast,
			JSONOutput: devValidateJSON,
		}

		if !opts.JSONOutput {
			ui.Section(fmt.Sprintf("Validating Dev Challenge: %s", challengeSlug))
		}

		if err := validateChallengeSlug(challengeSlug); err != nil {
			if !opts.JSONOutput {
				ui.Error("Invalid challenge slug")
			}
			return err
		}

		challengeDir := ""
		if devValidateDir != "" {
			dir, err := devutils.ResolveLocalChallengeDir(challengeSlug, devValidateDir)
			if err != nil {
				if !opts.JSONOutput {
					ui.Error("Failed to find challenge directory")
				}
				return err
			}
			challengeDir = dir
		}

		if devValidateWatch && devValidateWatchInterval <= 0 {
			return fmt.Errorf("--watch-interval must be a positive duration (e.g. 5s, 1m)")
		}

		if devValidateWatch {
			header := fmt.Sprintf("Validating Dev Challenge: %s (watch mode)", challengeSlug)
			return devutils.TickerWatchLoop(cmd.Context(), devValidateWatchInterval, header, func() {
				runDevValidate(cmd, challengeSlug, challengeDir, opts) //nolint:errcheck
			})
		}

		allPassed, err := runDevValidate(cmd, challengeSlug, challengeDir, opts)
		if err != nil {
			return err
		}

		if !allPassed {
			return fmt.Errorf("some validations failed")
		}

		return nil
	},
}

func init() {
	devCmd.AddCommand(devValidateCmd)
	devValidateCmd.Flags().StringVar(&devValidateDir, "dir", "", "Read from local directory")
	devValidateCmd.Flags().BoolVarP(&devValidateWatch, "watch", "w", false, "Continuously re-run validations at the given interval (see --watch-interval)")
	devValidateCmd.Flags().DurationVarP(&devValidateWatchInterval, "watch-interval", "i", 5*time.Second, "Interval between watch re-runs (e.g. 10s, 1m)")
	devValidateCmd.Flags().BoolVar(&devValidateFailFast, "fail-fast", false, "Stop at the first validation failure")
	devValidateCmd.Flags().BoolVar(&devValidateJSON, "json", false, "Output results as JSON")
}
