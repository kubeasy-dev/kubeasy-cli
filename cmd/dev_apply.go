package cmd

import (
	"fmt"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
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
	Short: "Deploy local challenge manifests to the Kind cluster",
	Long: `Deploys challenge manifests from a local directory to the Kind cluster.
This is the dev equivalent of 'kubeasy challenge start' but reads from the local
filesystem instead of pulling from the OCI registry. No API login required.

Use --clean to delete existing resources before applying (useful when manifests
have been removed and you want a fresh deploy).
Use --watch/-w to watch for file changes and auto-redeploy.`,
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

		if devApplyWatch {
			return runDevApplyWatch(cmd, challengeSlug, challengeDir)
		}

		ui.Info("Run 'kubeasy dev validate " + challengeSlug + "' to test your validations")
		return nil
	},
}

// runDevApplyWatch watches for file changes in the challenge directory and auto-redeploys.
func runDevApplyWatch(cmd *cobra.Command, challengeSlug, challengeDir string) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	defer watcher.Close()

	// Watch challenge.yaml and subdirectories
	watchPaths := []string{
		challengeDir,
		filepath.Join(challengeDir, "manifests"),
		filepath.Join(challengeDir, "policies"),
	}
	for _, p := range watchPaths {
		if err := watcher.Add(p); err != nil {
			// Non-fatal: directory might not exist
			ui.Warning(fmt.Sprintf("Cannot watch %s: %v", p, err))
		}
	}

	ui.Println()
	ui.Info("Watching for changes... Press Ctrl+C to stop")

	// Debounce: collect events and re-apply after 500ms of quiet
	var debounceTimer *time.Timer
	for {
		select {
		case <-ctx.Done():
			ui.Println()
			ui.Info("Watch mode stopped")
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) == 0 {
				continue
			}
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(500*time.Millisecond, func() {
				ui.Println()
				ui.Info(fmt.Sprintf("Change detected: %s", event.Name))
				if applyErr := runDevApply(cmd, challengeSlug, challengeDir, false); applyErr != nil {
					ui.Error(fmt.Sprintf("Re-apply failed: %v", applyErr))
				} else {
					ui.Success("Re-applied successfully")
				}
			})
		case watchErr, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			ui.Warning(fmt.Sprintf("Watcher error: %v", watchErr))
		}
	}
}

func init() {
	devCmd.AddCommand(devApplyCmd)
	devApplyCmd.Flags().StringVar(&devApplyDir, "dir", "", "Path to challenge directory (default: auto-detect)")
	devApplyCmd.Flags().BoolVar(&devApplyClean, "clean", false, "Delete existing resources before applying")
	devApplyCmd.Flags().BoolVarP(&devApplyWatch, "watch", "w", false, "Watch for file changes and auto-redeploy")
}
