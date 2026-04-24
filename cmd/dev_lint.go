package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kubeasy-dev/kubeasy-cli/internal/devutils"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation"
	"github.com/spf13/cobra"
)

var devLintCmd = &cobra.Command{
	Use:   "lint [challenge-slug]",
	Short: "Validate challenge.yaml structure without a cluster",
	Long: `Validates the structure and content of a challenge.yaml file.
Checks required fields, valid values, objective structure, and manifests directory.
No Kubernetes cluster is needed.

If a slug is given, it searches for challenge.yaml in the current directory 
or ../challenges/<slug>/. If no slug is given, it lints the current directory.`,
	Args:          cobra.MaximumNArgs(1),
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Section("Linting Challenge")

		var issues []devutils.LintIssue
		var err error
		var challengeYAML string

		if len(args) == 0 {
			// Lint current directory
			challengeYAML = "challenge.yaml"
			if _, err := os.Stat(challengeYAML); err != nil {
				ui.Error("No challenge.yaml found in current directory. Use a slug to specify another challenge.")
				return fmt.Errorf("challenge.yaml not found")
			}
		} else {
			slug := args[0]
			if err = validateChallengeSlug(slug); err != nil {
				ui.Error("Invalid challenge slug")
				return err
			}
			challengeYAML = validation.FindLocalChallengeFile(slug)
			if challengeYAML == "" {
				ui.Error(fmt.Sprintf("Failed to find local challenge file for slug %q", slug))
				return fmt.Errorf("challenge file not found")
			}
		}

		absPath, _ := filepath.Abs(challengeYAML)
		ui.Info(fmt.Sprintf("File: %s", absPath))
		ui.Println()

		issues, err = devutils.LintChallengeFile(challengeYAML)
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
			errCount, warnCount := 0, 0
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

		if len(issues) > 0 {
			ui.Warning(fmt.Sprintf("Found %d warning(s), no errors", len(issues)))
		} else {
			ui.Success("No issues found!")
		}

		return nil
	},
}

func init() {
	devCmd.AddCommand(devLintCmd)
}
