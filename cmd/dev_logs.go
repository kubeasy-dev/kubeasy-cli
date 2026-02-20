package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sync"

	"github.com/kubeasy-dev/kubeasy-cli/internal/devutils"
	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
	"github.com/kubeasy-dev/kubeasy-cli/internal/validation"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	devLogsDir       string
	devLogsFollow    bool
	devLogsContainer string
	devLogsTail      int64
)

var devLogsCmd = &cobra.Command{
	Use:   "logs [challenge-slug]",
	Short: "Stream logs from challenge pods",
	Long: `Shows logs from pods in a deployed challenge namespace.
Attempts to find relevant pods from challenge.yaml label selectors,
falling back to all pods in the namespace.

Use --follow/-f to stream logs continuously.
Use --container/-c to target a specific container.
Use --tail to control how many recent lines to show (default: 50).`,
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

		// Try to find pods from challenge.yaml label selectors
		pods, err := findChallengePods(ctx, clientset, challengeSlug, devLogsDir)
		if err != nil {
			return err
		}

		if len(pods) == 0 {
			ui.Warning("No pods found in namespace " + challengeSlug)
			return nil
		}

		ui.Info(fmt.Sprintf("Streaming logs from %d pod(s) in namespace '%s'", len(pods), challengeSlug))
		ui.Println()

		if len(pods) == 1 {
			return streamPodLogs(ctx, clientset, challengeSlug, pods[0].Name, devLogsContainer, devLogsTail, devLogsFollow)
		}

		// Multi-pod: stream in parallel with prefixes
		var wg sync.WaitGroup
		for _, pod := range pods {
			wg.Add(1)
			go func(podName string) {
				defer wg.Done()
				if err := streamPodLogsWithPrefix(ctx, clientset, challengeSlug, podName, devLogsContainer, devLogsTail, devLogsFollow); err != nil {
					pterm.Error.Printf("[%s] %v\n", podName, err)
				}
			}(pod.Name)
		}
		wg.Wait()

		return nil
	},
}

func findChallengePods(ctx context.Context, clientset kubernetes.Interface, slug, dirFlag string) ([]corev1.Pod, error) {
	// Try to extract selectors from challenge.yaml objectives
	challengeDir, dirErr := devutils.ResolveLocalChallengeDir(slug, dirFlag)
	if dirErr == nil {
		challengeYAML := filepath.Join(challengeDir, "challenge.yaml")
		config, parseErr := validation.LoadFromFile(challengeYAML)
		if parseErr == nil && len(config.Validations) > 0 {
			selectors := extractTargetSelectors(config.Validations)
			if len(selectors) > 0 {
				// Use first selector to find pods
				for _, sel := range selectors {
					labelStr := ""
					for k, v := range sel {
						if labelStr != "" {
							labelStr += ","
						}
						labelStr += k + "=" + v
					}
					pods, err := clientset.CoreV1().Pods(slug).List(ctx, metav1.ListOptions{
						LabelSelector: labelStr,
					})
					if err == nil && len(pods.Items) > 0 {
						return pods.Items, nil
					}
				}
			}
		}
	}

	// Fallback: all pods in namespace
	pods, err := clientset.CoreV1().Pods(slug).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}
	return pods.Items, nil
}

// extractTargetSelectors extracts label selectors from validation targets.
func extractTargetSelectors(validations []validation.Validation) []map[string]string {
	seen := make(map[string]bool)
	var selectors []map[string]string

	for _, v := range validations {
		var sel map[string]string
		switch spec := v.Spec.(type) {
		case validation.ConditionSpec:
			sel = spec.Target.LabelSelector
		case validation.StatusSpec:
			sel = spec.Target.LabelSelector
		case validation.LogSpec:
			sel = spec.Target.LabelSelector
		case validation.EventSpec:
			sel = spec.Target.LabelSelector
		case validation.ConnectivitySpec:
			sel = spec.SourcePod.LabelSelector
		}
		if len(sel) == 0 {
			continue
		}
		// Deduplicate
		key := fmt.Sprintf("%v", sel)
		if !seen[key] {
			seen[key] = true
			selectors = append(selectors, sel)
		}
	}
	return selectors
}

func streamPodLogs(ctx context.Context, clientset kubernetes.Interface, namespace, podName, container string, tail int64, follow bool) error {
	opts := &corev1.PodLogOptions{
		Follow:    follow,
		TailLines: &tail,
	}
	if container != "" {
		opts.Container = container
	}

	stream, err := clientset.CoreV1().Pods(namespace).GetLogs(podName, opts).Stream(ctx)
	if err != nil {
		return fmt.Errorf("failed to stream logs for pod %s: %w", podName, err)
	}
	defer stream.Close()

	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
	return scanner.Err()
}

func streamPodLogsWithPrefix(ctx context.Context, clientset kubernetes.Interface, namespace, podName, container string, tail int64, follow bool) error {
	opts := &corev1.PodLogOptions{
		Follow:    follow,
		TailLines: &tail,
	}
	if container != "" {
		opts.Container = container
	}

	stream, err := clientset.CoreV1().Pods(namespace).GetLogs(podName, opts).Stream(ctx)
	if err != nil {
		return fmt.Errorf("failed to stream logs for pod %s: %w", podName, err)
	}
	defer func() { _ = stream.Close() }()

	reader := bufio.NewReader(stream)
	prefix := pterm.LightCyan(fmt.Sprintf("[%s] ", podName))
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			fmt.Printf("%s%s", prefix, line)
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func init() {
	devCmd.AddCommand(devLogsCmd)
	devLogsCmd.Flags().StringVar(&devLogsDir, "dir", "", "Path to challenge directory (default: auto-detect)")
	devLogsCmd.Flags().BoolVarP(&devLogsFollow, "follow", "f", false, "Follow log output")
	devLogsCmd.Flags().StringVarP(&devLogsContainer, "container", "c", "", "Target a specific container")
	devLogsCmd.Flags().Int64Var(&devLogsTail, "tail", 50, "Number of recent log lines to show")
}
