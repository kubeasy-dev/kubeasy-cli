package cmd

import (
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/devutils"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	devTestDir      string
	devTestClean    bool
	devTestWatch    bool
	devTestFailFast bool
	devTestJSON     bool
)

var devTestCmd = &cobra.Command{
	Use:   "test [challenge-slug]",
	Short: "Apply manifests and run validations (apply + validate)",
	Long: `Deploys challenge manifests and then runs validations in one step.
This is equivalent to running 'kubeasy dev apply' followed by 'kubeasy dev validate'.

Use --clean to delete existing resources before applying.
Use --watch to continuously re-run validations every 5 seconds after the initial apply.
Use --fail-fast to stop at the first validation failure.
Use --json for structured JSON output (useful for CI).`,
	Args:          cobra.ExactArgs(1),
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		challengeSlug := args[0]

		opts := DevValidateOpts{
			FailFast:   devTestFailFast,
			JSONOutput: devTestJSON,
		}

		// Validate slug format
		if err := validateChallengeSlug(challengeSlug); err != nil {
			if !opts.JSONOutput {
				ui.Error("Invalid challenge slug")
			}
			return err
		}

		// Resolve local challenge directory
		challengeDir, err := devutils.ResolveLocalChallengeDir(challengeSlug, devTestDir)
		if err != nil {
			if !opts.JSONOutput {
				ui.Error("Failed to find challenge directory")
			}
			return err
		}

		// Apply
		ui.Section(fmt.Sprintf("Applying Dev Challenge: %s", challengeSlug))
		ui.Info(fmt.Sprintf("Using challenge directory: %s", challengeDir))

		if err := runDevApply(cmd, challengeSlug, challengeDir, devTestClean); err != nil {
			return err
		}

		ui.Println()
		ui.Success(fmt.Sprintf("Challenge '%s' deployed from local files!", challengeSlug))
		ui.Println()

		// Validate
		if !opts.JSONOutput {
			ui.Section(fmt.Sprintf("Validating Dev Challenge: %s", challengeSlug))
		}

		if devTestWatch {
			return runDevTestWatch(cmd, challengeSlug, challengeDir, opts)
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

// runDevTestWatch runs validations in a loop every 5 seconds after the initial apply.
func runDevTestWatch(cmd *cobra.Command, challengeSlug, challengeDir string, opts DevValidateOpts) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Run immediately on first iteration
	runDevValidate(cmd, challengeSlug, challengeDir, opts) //nolint:errcheck

	for {
		select {
		case <-ctx.Done():
			ui.Println()
			ui.Info("Watch mode stopped")
			return nil
		case <-ticker.C:
			fmt.Print("\033[H\033[2J")
			ui.Section(fmt.Sprintf("Validating Dev Challenge: %s (watch mode)", challengeSlug))
			ui.Info(fmt.Sprintf("Last run: %s — Press Ctrl+C to stop", time.Now().Format("15:04:05")))
			ui.Println()
			runDevValidate(cmd, challengeSlug, challengeDir, opts) //nolint:errcheck
		}
	}
}

func init() {
	devCmd.AddCommand(devTestCmd)
	devTestCmd.Flags().StringVar(&devTestDir, "dir", "", "Path to challenge directory (default: auto-detect)")
	devTestCmd.Flags().BoolVar(&devTestClean, "clean", false, "Delete existing resources before applying")
	devTestCmd.Flags().BoolVarP(&devTestWatch, "watch", "w", false, "Continuously re-run validations every 5 seconds after apply")
	devTestCmd.Flags().BoolVar(&devTestFailFast, "fail-fast", false, "Stop at the first validation failure")
	devTestCmd.Flags().BoolVar(&devTestJSON, "json", false, "Output results as JSON")
}
