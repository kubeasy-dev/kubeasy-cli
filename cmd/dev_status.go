package cmd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/internal/devutils"
	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var devStatusDir string

var devStatusCmd = &cobra.Command{
	Use:   "status [challenge-slug]",
	Short: "Show current challenge state at a glance",
	Long: `Displays pods, recent events, and objective count for a deployed challenge.
Requires the challenge to be deployed in the Kind cluster.`,
	Args:          cobra.ExactArgs(1),
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		challengeSlug := args[0]

		if err := validateChallengeSlug(challengeSlug); err != nil {
			ui.Error("Invalid challenge slug")
			return err
		}

		clientset, err := kube.GetKubernetesClient()
		if err != nil {
			ui.Error("Failed to get Kubernetes client. Is the cluster running? Try 'kubeasy setup'")
			return fmt.Errorf("failed to get Kubernetes client: %w", err)
		}

		ctx := cmd.Context()

		ui.Section(fmt.Sprintf("Challenge Status: %s", challengeSlug))

		// Check namespace exists
		_, err = clientset.CoreV1().Namespaces().Get(ctx, challengeSlug, metav1.GetOptions{})
		if err != nil {
			ui.Error(fmt.Sprintf("Namespace '%s' not found. Is the challenge deployed?", challengeSlug))
			return fmt.Errorf("namespace not found: %w", err)
		}

		// List pods
		pods, err := clientset.CoreV1().Pods(challengeSlug).List(ctx, metav1.ListOptions{})
		if err != nil {
			ui.Error("Failed to list pods")
			return fmt.Errorf("failed to list pods: %w", err)
		}

		ui.Section("Pods")
		if len(pods.Items) == 0 {
			ui.Info("No pods found in namespace")
		} else {
			rows := make([][]string, 0, len(pods.Items))
			for _, pod := range pods.Items {
				readyCount := 0
				total := len(pod.Spec.Containers)
				for _, cs := range pod.Status.ContainerStatuses {
					if cs.Ready {
						readyCount++
					}
				}
				ready := fmt.Sprintf("%d/%d", readyCount, total)

				restarts := int32(0)
				for _, cs := range pod.Status.ContainerStatuses {
					restarts += cs.RestartCount
				}

				age := time.Since(pod.CreationTimestamp.Time).Round(time.Second)

				rows = append(rows, []string{
					pod.Name,
					string(pod.Status.Phase),
					ready,
					fmt.Sprintf("%d", restarts),
					formatAge(age),
				})
			}
			if err := ui.Table([]string{"NAME", "STATUS", "READY", "RESTARTS", "AGE"}, rows); err != nil {
				return fmt.Errorf("failed to render table: %w", err)
			}
		}

		// List recent events (last 5 minutes, max 10)
		events, err := clientset.CoreV1().Events(challengeSlug).List(ctx, metav1.ListOptions{})
		if err != nil {
			ui.Warning(fmt.Sprintf("Failed to list events: %v", err))
		} else {
			since := time.Now().Add(-5 * time.Minute)
			var recentRows [][]string
			for _, event := range events.Items {
				eventTime := event.LastTimestamp.Time
				if eventTime.IsZero() {
					eventTime = event.EventTime.Time
				}
				if eventTime.Before(since) {
					continue
				}
				recentRows = append(recentRows, []string{
					formatAge(time.Since(eventTime).Round(time.Second)),
					event.Type,
					event.Reason,
					truncate(event.Message, 60),
				})
				if len(recentRows) >= 10 {
					break
				}
			}

			ui.Println()
			ui.Section("Recent Events (last 5m)")
			if len(recentRows) == 0 {
				ui.Info("No recent events")
			} else {
				if err := ui.Table([]string{"AGE", "TYPE", "REASON", "MESSAGE"}, recentRows); err != nil {
					return fmt.Errorf("failed to render table: %w", err)
				}
			}
		}

		// Best-effort: count objectives from challenge.yaml
		challengeDir, dirErr := devutils.ResolveLocalChallengeDir(challengeSlug, devStatusDir)
		if dirErr == nil {
			challengeYAML := filepath.Join(challengeDir, "challenge.yaml")
			config, parseErr := validation.LoadFromFile(challengeYAML)
			if parseErr == nil {
				ui.Println()
				ui.Info(fmt.Sprintf("Objectives defined: %d", len(config.Validations)))
			}
		}

		return nil
	},
}

func formatAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func init() {
	devCmd.AddCommand(devStatusCmd)
	devStatusCmd.Flags().StringVar(&devStatusDir, "dir", "", "Path to challenge directory (default: auto-detect)")
}
