package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	operator "github.com/kubeasy-dev/challenge-operator/api/v1alpha1"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/api"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/kube"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var submitCmd = &cobra.Command{
	Use:   "submit [challenge-slug]",
	Short: "Submit a challenge solution",
	Long: `Submit a challenge solution to Kubeasy. This command will package your solution
and send it to the Kubeasy API for evaluation. Make sure you have completed the challenge before submitting.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		challengeSlug := args[0]
		fmt.Printf("Submitting solution for challenge: %s\n", challengeSlug)

		challenge, err := api.GetChallenge(challengeSlug)
		if err != nil {
			log.Fatalf("Error fetching challenge: %v", err)
		}

		progress, err := api.GetChallengeProgress(challengeSlug)
		if err != nil {
			log.Fatalf("Error fetching user progress: %v", err)
		}
		if progress == nil {
			fmt.Println("❌ No user_progress found for this challenge. Please start the challenge first.")
			return
		}
		if progress.Status == "completed" {
			fmt.Printf("❌ This challenge is already completed for this user. Submission is not allowed. You can reset the challenge with 'kubeasy challenge reset %s'.\n", challengeSlug)
			return
		}

		dynamicClient, err := kube.GetDynamicClient()
		if err != nil {
			log.Fatalf("Error getting dynamic client: %v", err)
		}

		svGVR := schema.GroupVersionResource{
			Group:    "challenge.kubeasy.dev",
			Version:  "v1alpha1",
			Resource: "staticvalidations",
		}

		dvGVR := schema.GroupVersionResource{
			Group:    "challenge.kubeasy.dev",
			Version:  "v1alpha1",
			Resource: "dynamicvalidations",
		}

		namespace := challengeSlug

		svListUnstructured, err := dynamicClient.Resource(svGVR).Namespace(namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			log.Fatalf("Error listing StaticValidations in namespace %s: %v", namespace, err)
		}

		if len(svListUnstructured.Items) == 0 {
			fmt.Printf("No StaticValidations found in namespace %s. Cannot verify submission.\n", namespace)
			return
		}

		allStaticSucceeded := true
		allDynamicSucceeded := true
		detailedStatuses := map[string]interface{}{
			"staticValidations":  map[string][]operator.StaticValidationStatus{},
			"dynamicValidations": map[string][]operator.DynamicValidationStatus{},
		}

		for _, svUnstructured := range svListUnstructured.Items {
			var sv operator.StaticValidation
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(svUnstructured.Object, &sv)
			if err != nil {
				log.Fatalf("Error converting StaticValidation to StaticValidation: %v", err)
			}

			staticStatuses := detailedStatuses["staticValidations"].(map[string][]operator.StaticValidationStatus)
			staticStatuses[svUnstructured.GetName()] = []operator.StaticValidationStatus{sv.Status}

			if !sv.Status.AllPassed {
				allStaticSucceeded = false
			}
		}

		// --- Check DynamicValidations ---
		dvListUnstructured, err := dynamicClient.Resource(dvGVR).Namespace(namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			log.Printf("Error listing DynamicValidations in namespace %s: %v", namespace, err)
			// Continue with the submission process even if there's an error with DynamicValidations
			allDynamicSucceeded = false
		} else {
			// Process DynamicValidations if any exist
			if len(dvListUnstructured.Items) > 0 {
				for _, dvUnstructured := range dvListUnstructured.Items {
					var dv operator.DynamicValidation
					err = runtime.DefaultUnstructuredConverter.FromUnstructured(dvUnstructured.Object, &dv)
					if err != nil {
						log.Printf("Error converting DynamicValidation to DynamicValidation: %v", err)
						allDynamicSucceeded = false
						continue
					}

					dynamicStatuses := detailedStatuses["dynamicValidations"].(map[string][]operator.DynamicValidationStatus)
					dynamicStatuses[dvUnstructured.GetName()] = []operator.DynamicValidationStatus{dv.Status}

					if !dv.Status.AllPassed {
						allDynamicSucceeded = false
					}
				}
			}
		}

		// --- Report Result & Call API ---
		if allStaticSucceeded && allDynamicSucceeded {
			fmt.Println("\n✅ All validations succeeded!")
			err = api.SendSubmit(challenge.Id, true, true, detailedStatuses)
		} else if allStaticSucceeded && !allDynamicSucceeded {
			fmt.Println("\n✅ All StaticValidations succeeded!")
			fmt.Println("❌ Some DynamicValidations did not succeed or encountered errors.")
			err = api.SendSubmit(challenge.Id, true, false, detailedStatuses)
		} else if !allStaticSucceeded && allDynamicSucceeded {
			fmt.Println("\n❌ Some StaticValidations did not succeed or encountered errors.")
			fmt.Println("✅ All DynamicValidations succeeded!")
			err = api.SendSubmit(challenge.Id, false, true, detailedStatuses)
		} else {
			fmt.Println("\n❌ Some StaticValidations did not succeed or encountered errors.")
			fmt.Println("❌ Some DynamicValidations did not succeed or encountered errors.")
			err = api.SendSubmit(challenge.Id, false, false, detailedStatuses)
		}

		if err != nil {
			log.Printf("Error sending submission: %v", err)
			os.Exit(1) // Envisager de sortir avec une erreur
		}
	},
}

func init() {
	challengeCmd.AddCommand(submitCmd)
}
