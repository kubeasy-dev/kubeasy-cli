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

var cleanChallengeCmd = &cobra.Command{
	Use:   "clean [challenge-slug]",
	Short: "Clean a challenge",
	Long:  `Cleans a challenge by removing challenge all associated resources`,
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

		fmt.Printf("Challenge '%s' cleaned successfully.\n", challengeSlug)
	},
}

func init() {
	challengeCmd.AddCommand(cleanChallengeCmd)
}
