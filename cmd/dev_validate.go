package cmd

import (
	"fmt"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/devutils"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	devValidateDir      string
	devValidateWatch    bool
	devValidateFailFast bool
	devValidateJSON     bool
)

var devValidateCmd = &cobra.Command{
	Use:   "validate [challenge-slug]",
	Short: "Run validations locally without submitting to API",
	Long: `Runs challenge validations against the Kind cluster and displays results.
This is the dev equivalent of 'kubeasy challenge submit' but does not send
results to the Kubeasy API. No login required.

Use --watch to continuously re-run validations every 5 seconds.
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

		// Validate slug format
		if err := validateChallengeSlug(challengeSlug); err != nil {
			if !opts.JSONOutput {
				ui.Error("Invalid challenge slug")
			}
			return err
		}

		// Resolve local challenge directory
		challengeDir, err := devutils.ResolveLocalChallengeDir(challengeSlug, devValidateDir)
		if err != nil {
			if !opts.JSONOutput {
				ui.Error("Failed to find challenge directory")
			}
			return err
		}

		if devValidateWatch {
			header := fmt.Sprintf("Validating Dev Challenge: %s (watch mode)", challengeSlug)
			return devutils.TickerWatchLoop(cmd.Context(), 5*time.Second, header, func() {
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
	devValidateCmd.Flags().StringVar(&devValidateDir, "dir", "", "Path to challenge directory (default: auto-detect)")
	devValidateCmd.Flags().BoolVarP(&devValidateWatch, "watch", "w", false, "Continuously re-run validations every 5 seconds")
	devValidateCmd.Flags().BoolVar(&devValidateFailFast, "fail-fast", false, "Stop at the first validation failure")
	devValidateCmd.Flags().BoolVar(&devValidateJSON, "json", false, "Output results as JSON")
}
