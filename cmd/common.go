package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/api"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/argocd"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/kube"
)


// getChallengeOrExit tries to get a challenge and exits with an error if it fails
func getChallengeOrExit(slug string) *api.ChallengeEntity {
	challenge, err := api.GetChallenge(slug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching challenge: %v\n", err)
		os.Exit(1)
	}
	if challenge == nil {
		fmt.Fprintf(os.Stderr, "Challenge '%s' not found.\n", slug)
		os.Exit(1)
	}
	return challenge
}

// deleteChallengeResources deletes ArgoCD Application and all subresources for a challenge
func deleteChallengeResources(challengeSlug string) {
	challenge := getChallengeOrExit(challengeSlug)
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
}
