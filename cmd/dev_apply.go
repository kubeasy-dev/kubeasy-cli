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
)

var devApplyCmd = &cobra.Command{
	Use:   "apply [challenge-slug]",
	Short: "Deploy local challenge manifests to the Kind cluster",
	Long: `Deploys challenge manifests from a local directory to the Kind cluster.
This is the dev equivalent of 'kubeasy challenge start' but reads from the local
filesystem instead of pulling from the OCI registry. No API login required.

Use --clean to delete existing resources before applying (useful when manifests
have been removed and you want a fresh deploy).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		challengeSlug := args[0]

		ui.Section(fmt.Sprintf("Applying Dev Challenge: %s", challengeSlug))

		// Validate slug format
		if err := validateChallengeSlug(challengeSlug); err != nil {
			ui.Error("Invalid challenge slug")
			return err
		}

		// Resolve local challenge directory
		challengeDir, err := devutils.ResolveLocalChallengeDir(challengeSlug, devApplyDir)
		if err != nil {
			ui.Error("Failed to find challenge directory")
			return err
		}
		ui.Info(fmt.Sprintf("Using challenge directory: %s", challengeDir))

		if err := runDevApply(cmd, challengeSlug, challengeDir, devApplyClean); err != nil {
			return err
		}

		ui.Println()
		ui.Success(fmt.Sprintf("Challenge '%s' deployed from local files!", challengeSlug))
		ui.Info(fmt.Sprintf("Namespace: %s", challengeSlug))
		ui.Info("Run 'kubeasy dev validate " + challengeSlug + "' to test your validations")

		return nil
	},
}

func init() {
	devCmd.AddCommand(devApplyCmd)
	devApplyCmd.Flags().StringVar(&devApplyDir, "dir", "", "Path to challenge directory (default: auto-detect)")
	devApplyCmd.Flags().BoolVar(&devApplyClean, "clean", false, "Delete existing resources before applying")
}
