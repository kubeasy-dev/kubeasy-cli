package devutils

import (
	"context"
	"fmt"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
)

// TickerWatchLoop runs fn immediately, then repeats every interval with screen clear.
// Stops on SIGINT/SIGTERM. header is displayed at the top of each iteration.
func TickerWatchLoop(ctx context.Context, interval time.Duration, header string, fn func()) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run immediately
	ui.ClearScreen()
	ui.Section(header)
	ui.Info(fmt.Sprintf("Last run: %s — Press Ctrl+C to stop", time.Now().Format("15:04:05")))
	ui.Println()
	fn()

	for {
		select {
		case <-ctx.Done():
			ui.Println()
			ui.Info("Watch mode stopped")
			return nil
		case <-ticker.C:
			ui.ClearScreen()
			ui.Section(header)
			ui.Info(fmt.Sprintf("Last run: %s — Press Ctrl+C to stop", time.Now().Format("15:04:05")))
			ui.Println()
			fn()
		}
	}
}

// FsWatchLoop watches challengeDir (and manifests/, policies/ subdirs) for changes.
// On each change (debounced), calls onChange. Stops on SIGINT/SIGTERM.
func FsWatchLoop(ctx context.Context, challengeDir string, onChange func()) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	defer watcher.Close()

	watchPaths := []string{
		challengeDir,
		filepath.Join(challengeDir, "manifests"),
		filepath.Join(challengeDir, "policies"),
	}
	for _, p := range watchPaths {
		if err := watcher.Add(p); err != nil {
			ui.Warning(fmt.Sprintf("Cannot watch %s: %v", p, err))
		}
	}

	ui.Println()
	ui.Info("Watching for changes... Press Ctrl+C to stop")

	redeployCount := 0
	var debounceTimer *time.Timer
	for {
		select {
		case <-ctx.Done():
			ui.Println()
			ui.Info(fmt.Sprintf("Watch mode stopped (%d redeploy(s))", redeployCount))
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
				redeployCount++
				ui.Println()
				ui.Info(fmt.Sprintf("Change detected: %s (redeploy #%d)", event.Name, redeployCount))
				onChange()
			})
		case watchErr, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			ui.Warning(fmt.Sprintf("Watcher error: %v", watchErr))
		}
	}
}
