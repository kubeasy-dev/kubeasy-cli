package cmd

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/deployer"
	"github.com/kubeasy-dev/kubeasy-cli/internal/devutils"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/spf13/cobra"
)

var (
	devApplyDir      string
	devApplyClean    bool
	devApplyWatch    bool
	devApplyInterval time.Duration
)

var devApplyCmd = &cobra.Command{
	Use:   "apply [challenge-slug]",
	Short: "Deploy challenge manifests from the local registry to the Kind cluster",
	Long: `Fetches manifests from the local registry (http://localhost:8080 by default)
and deploys them to the Kind cluster. This is the dev equivalent of 'kubeasy challenge start'.

Use --dir to read from a local directory instead of the registry (useful when
the challenge has a custom image/ that needs to be built locally).
Use --clean to delete existing resources before applying.
Use --watch/-w to watch for changes and auto-redeploy (polls registry, or uses fsnotify with --dir).`,
	Args:          cobra.ExactArgs(1),
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		challengeSlug := args[0]

		ui.Section(fmt.Sprintf("Applying Dev Challenge: %s", challengeSlug))

		if err := validateChallengeSlug(challengeSlug); err != nil {
			ui.Error("Invalid challenge slug")
			return err
		}

		// Resolve filesystem dir if --dir provided; otherwise use registry mode.
		challengeDir := ""
		if devApplyDir != "" {
			dir, err := devutils.ResolveLocalChallengeDir(challengeSlug, devApplyDir)
			if err != nil {
				ui.Error("Failed to find challenge directory")
				return err
			}
			challengeDir = dir
			ui.Info(fmt.Sprintf("Using local directory: %s", challengeDir))
		} else {
			ui.Info(fmt.Sprintf("Using registry: %s", devRegistryURL))
		}

		if _, err := runDevApply(cmd, challengeSlug, challengeDir, devRegistryURL, devApplyClean); err != nil {
			return err
		}

		ui.Println()
		ui.Success(fmt.Sprintf("Challenge '%s' deployed!", challengeSlug))

		if devApplyWatch {
			if challengeDir != "" {
				// Filesystem mode: use fsnotify for instant feedback.
				return devutils.FsWatchLoop(cmd.Context(), challengeDir, func() {
					if _, err := runDevApply(cmd, challengeSlug, challengeDir, devRegistryURL, false); err != nil {
						ui.Error(fmt.Sprintf("Re-apply failed: %v", err))
					} else {
						ui.Success("Re-applied successfully")
					}
				})
			}
			// Registry mode: poll for manifest hash changes.
			return registryPollLoop(cmd.Context(), challengeSlug, devRegistryURL, devApplyInterval, func() {
				if _, err := runDevApply(cmd, challengeSlug, "", devRegistryURL, false); err != nil {
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

// registryPollLoop polls the registry every interval and calls onChange when manifest content changes.
func registryPollLoop(ctx context.Context, slug, registryURL string, interval time.Duration, onChange func()) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	lastHash, err := deployer.FetchManifestHash(ctx, registryURL, slug)
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not fetch initial hash: %v", err))
	}

	ui.Println()
	ui.Info(fmt.Sprintf("Watching registry for changes every %s... Press Ctrl+C to stop", interval))

	for {
		select {
		case <-ctx.Done():
			ui.Println()
			ui.Info("Watch mode stopped")
			return nil
		case <-ticker.C:
			hash, err := deployer.FetchManifestHash(ctx, registryURL, slug)
			if err != nil {
				ui.Warning(fmt.Sprintf("Failed to check registry: %v", err))
				continue
			}
			if hash == lastHash {
				continue
			}
			lastHash = hash
			ui.Println()
			ui.Info("Change detected in registry, re-applying...")
			onChange()
		}
	}
}

func init() {
	devCmd.AddCommand(devApplyCmd)
	devApplyCmd.Flags().StringVar(&devApplyDir, "dir", "", "Read from local directory instead of registry")
	devApplyCmd.Flags().BoolVar(&devApplyClean, "clean", false, "Delete existing resources before applying")
	devApplyCmd.Flags().BoolVarP(&devApplyWatch, "watch", "w", false, "Watch for changes and auto-redeploy")
	devApplyCmd.Flags().DurationVar(&devApplyInterval, "watch-interval", 2*time.Second, "Polling interval for registry watch mode (ignored with --dir)")
}
