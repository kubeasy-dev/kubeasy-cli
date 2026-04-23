package cmd

import (
	"fmt"
	"io"
	"net/http"
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
No Kubernetes cluster is needed.

When a slug is given and --dir is not set, fetches challenge.yaml from the local
registry (http://localhost:8080 by default). Use --dir to read from a local path instead.`,
	Args:          cobra.MaximumNArgs(1),
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Section("Linting Challenge")

		var issues []devutils.LintIssue
		var err error

		if devLintDir != "" || len(args) == 0 {
			// Filesystem mode: --dir flag provided, or no slug given (lint current directory).
			var challengeDir string
			if devLintDir != "" {
				challengeDir, err = filepath.Abs(devLintDir)
				if err != nil {
					return fmt.Errorf("failed to resolve path: %w", err)
				}
			} else {
				challengeDir, err = devutils.ResolveLocalChallengeDir("", "")
				if err != nil {
					ui.Error("Failed to find challenge directory. Use a slug or --dir to specify.")
					return err
				}
			}

			challengeYAML := filepath.Join(challengeDir, "challenge.yaml")
			ui.Info(fmt.Sprintf("File: %s", challengeYAML))
			ui.Println()

			issues, err = devutils.LintChallengeFile(challengeYAML)
		} else {
			// Registry mode: fetch YAML from local registry.
			slug := args[0]
			if err = validateChallengeSlug(slug); err != nil {
				ui.Error("Invalid challenge slug")
				return err
			}

			url := fmt.Sprintf("%s/challenges/%s/yaml", devRegistryURL, slug)
			ui.Info(fmt.Sprintf("Fetching: %s", url))
			ui.Println()

			resp, httpErr := http.Get(url) //nolint:noctx,gosec
			if httpErr != nil {
				ui.Error(fmt.Sprintf("Failed to reach registry: %v", httpErr))
				return httpErr
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("registry returned HTTP %d for challenge %q", resp.StatusCode, slug)
			}

			data, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				return fmt.Errorf("failed to read response: %w", readErr)
			}

			issues, err = devutils.LintChallengeData(data)
		}

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
	devLintCmd.Flags().StringVar(&devLintDir, "dir", "", "Read from local directory instead of registry")
}
