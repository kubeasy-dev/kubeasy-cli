package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/api"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/argocd"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/kube"
	"github.com/spf13/cobra"
)

var resetChallengeCmd = &cobra.Command{
	Use:   "reset [challenge-slug]",
	Short: "Reset a challenge",
	Long:  `Resets a challenge by removing challenge namespace and resetting progress and submissions`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		challengeSlug := args[0]

		challenge, err := api.GetChallenge(challengeSlug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching challenge: %v\n", err)
			os.Exit(1)
		}

		// Delete ArgoCD Application and all subresources
		dynamicClient, err := kube.GetDynamicClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting Kubernetes dynamic client: %v\n", err)
			os.Exit(1)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := argocd.DeleteChallengeApplication(ctx, dynamicClient, challenge.Slug, argocd.ArgoCDNamespace); err != nil {
			fmt.Fprintf(os.Stderr, "Error deleting ArgoCD Application for challenge '%s': %v\n", challengeSlug, err)
			os.Exit(1)
		}

		// Reset challenge progress
		if err := api.ResetChallengeProgress(challenge.Id); err != nil {
			fmt.Fprintf(os.Stderr, "Error resetting challenge '%s': %v\n", challengeSlug, err)
			os.Exit(1)
		}

		fmt.Printf("Challenge '%s' reset successfully (including ArgoCD app and subresources).\n", challengeSlug)
	},
}

func init() {
	challengeCmd.AddCommand(resetChallengeCmd)
}
