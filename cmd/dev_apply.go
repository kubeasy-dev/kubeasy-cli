package cmd

import (
	"fmt"

	"github.com/kubeasy-dev/kubeasy-cli/internal/devutils"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	devApplyDir   string
	devApplyClean bool
	devApplyWatch bool
)

var devApplyCmd = &cobra.Command{
	Use:   "apply [challenge-slug]",
	Short: "Deploy challenge manifests from local files to the Kind cluster",
	Long: `Deploys challenge manifests from local files to the Kind cluster.
This is the dev equivalent of 'kubeasy challenge start'.

It searches for challenge.yaml in the current directory or ../challenges/<slug>/.
Use --dir to specify a custom directory.
Use --clean to delete existing resources before applying.
Use --watch/-w to watch for changes and auto-redeploy (uses fsnotify).`,
	Args:          cobra.ExactArgs(1),
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		challengeSlug := args[0]

		ui.Section(fmt.Sprintf("Applying Dev Challenge: %s", challengeSlug))

		if err := validateChallengeSlug(challengeSlug); err != nil {
			ui.Error("Invalid challenge slug")
			return err
		}

		challengeDir := ""
		if devApplyDir != "" {
			dir, err := devutils.ResolveLocalChallengeDir(challengeSlug, devApplyDir)
			if err != nil {
				ui.Error("Failed to find challenge directory")
				return err
			}
			challengeDir = dir
			ui.Info(fmt.Sprintf("Using local directory: %s", challengeDir))
		}

		if err := runDevApply(cmd, challengeSlug, challengeDir, devApplyClean); err != nil {
			return err
		}

		ui.Println()
		ui.Success(fmt.Sprintf("Challenge '%s' deployed!", challengeSlug))

		if devApplyWatch {
			// Resolve directory if not already set (needed for watch)
			if challengeDir == "" {
				dir, err := devutils.ResolveLocalChallengeDir(challengeSlug, "")
				if err != nil {
					return fmt.Errorf("failed to resolve challenge directory for watch: %w", err)
				}
				challengeDir = dir
			}

			return devutils.FsWatchLoop(cmd.Context(), challengeDir, func() {
				if err := runDevApply(cmd, challengeSlug, challengeDir, false); err != nil {
					ui.Error(fmt.Sprintf("Re-apply failed: %v", err))
				} else {
					ui.Success("Re-applied successfully")
				}
			})
		}

		ui.Info("Run 'kubeasy dev validate " + challengeSlug + "' to test your validations")
		return nil
	},
}

func init() {
	devCmd.AddCommand(devApplyCmd)
	devApplyCmd.Flags().StringVar(&devApplyDir, "dir", "", "Read from local directory")
	devApplyCmd.Flags().BoolVar(&devApplyClean, "clean", false, "Delete existing resources before applying")
	devApplyCmd.Flags().BoolVarP(&devApplyWatch, "watch", "w", false, "Watch for changes and auto-redeploy")
}
