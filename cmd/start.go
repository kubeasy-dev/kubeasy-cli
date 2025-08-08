package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/api"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/argocd"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/constants"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/kube"
	"github.com/spf13/cobra"
)

var startChallengeCmd = &cobra.Command{
	Use:   "start [challenge-slug]",
	Short: "Start a challenge",
	Long:  `Starts a challenge by installing the necessary components into the local Kubernetes cluster.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		challengeSlug := args[0]
		_, err := api.GetChallenge(challengeSlug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching challenge: %v\n", err)
			os.Exit(1)
		}

		progress, err := api.GetChallengeProgress(challengeSlug)

		if progress != nil {
			fmt.Printf("Challenge already started. Continue the challenge or you can reset it with 'kubeasy challenge reset %s'\n", challengeSlug)
			os.Exit(0)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching challenge progress: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Installing ArgoCD app for challenge...")
		ctx := context.Background()
		dynamicClient, err := kube.GetDynamicClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting Kubernetes dynamic client: %v\n", err)
			os.Exit(1)
		}
		if err := argocd.CreateOrUpdateChallengeApplication(ctx, dynamicClient, challengeSlug); err != nil {
			fmt.Fprintf(os.Stderr, "Error installing ArgoCD app: %v\n", err)
			os.Exit(1)
		}
		_ = kube.SetNamespaceForContext(constants.KubeasyClusterContext, challengeSlug)
		fmt.Printf("Kubernetes context set to 'kind-kubeasy' and namespace to '%s' \n", challengeSlug)
		if err := api.StartChallenge(challengeSlug); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting challenge: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Challenge environment is ready!")
		fmt.Printf("You can now start working on the challenge '%s'", challengeSlug)
	},
}

func init() {
	challengeCmd.AddCommand(startChallengeCmd)
}
