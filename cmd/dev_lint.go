package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/kubeasy-dev/kubeasy-cli/internal/devutils"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/spf13/cobra"
)

var devLintDir string

var devLintCmd = &cobra.Command{
	Use:   "lint [challenge-slug]",
	Short: "Validate challenge.yaml structure without a cluster",
	Long: `Validates the structure and content of a challenge.yaml file.
Checks required fields, valid values, objective structure, and manifests directory.
No Kubernetes cluster is needed.`,
	Args:          cobra.MaximumNArgs(1),
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		var challengeDir string

		if len(args) > 0 {
			slug := args[0]
			if err := validateChallengeSlug(slug); err != nil {
				ui.Error("Invalid challenge slug")
				return err
			}
			dir, err := devutils.ResolveLocalChallengeDir(slug, devLintDir)
			if err != nil {
				ui.Error("Failed to find challenge directory")
				return err
			}
			challengeDir = dir
		} else if devLintDir != "" {
			absDir, err := filepath.Abs(devLintDir)
			if err != nil {
				return fmt.Errorf("failed to resolve path: %w", err)
			}
			challengeDir = absDir
		} else {
			// Try current directory
			absDir, err := filepath.Abs(".")
			if err != nil {
				return fmt.Errorf("failed to resolve path: %w", err)
			}
			challengeDir = absDir
		}

		challengeYAML := filepath.Join(challengeDir, "challenge.yaml")
		ui.Section("Linting Challenge")
		ui.Info(fmt.Sprintf("File: %s", challengeYAML))
		ui.Println()

		issues, err := devutils.LintChallengeFile(challengeYAML)
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to lint: %v", err))
			return err
		}

		hasErrors := false
		for _, issue := range issues {
			switch issue.Severity {
			case devutils.SeverityError:
				hasErrors = true
				ui.Error(fmt.Sprintf("[%s] %s", issue.Field, issue.Message))
			case devutils.SeverityWarning:
				ui.Warning(fmt.Sprintf("[%s] %s", issue.Field, issue.Message))
			}
		}

		ui.Println()
		if hasErrors {
			errCount := 0
			warnCount := 0
			for _, issue := range issues {
				if issue.Severity == devutils.SeverityError {
					errCount++
				} else {
					warnCount++
				}
			}
			ui.Error(fmt.Sprintf("Found %d error(s) and %d warning(s)", errCount, warnCount))
			return fmt.Errorf("lint failed with %d error(s)", errCount)
		}

		warnCount := len(issues)
		if warnCount > 0 {
			ui.Warning(fmt.Sprintf("Found %d warning(s), no errors", warnCount))
		} else {
			ui.Success("No issues found!")
		}

		return nil
	},
}

func init() {
	devCmd.AddCommand(devLintCmd)
	devLintCmd.Flags().StringVar(&devLintDir, "dir", "", "Path to challenge directory (default: auto-detect)")
}
