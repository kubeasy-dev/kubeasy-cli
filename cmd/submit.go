package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	operator "github.com/kubeasy-dev/challenge-operator/api/v1alpha1"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/api"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
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

		// --- Kubernetes Client Initialization ---
		kubeconfigPath := filepath.Join(os.Getenv("HOME"), ".kube", "config") // Adjust if needed
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			log.Fatalf("Error building kubeconfig: %v", err)
		}

		dynamicClient, err := dynamic.NewForConfig(config)
		if err != nil {
			log.Fatalf("Error creating dynamic client: %v", err)
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
			return // Ou peut-être que c'est une réussite ? À définir.
		}

		allStaticSucceeded := true
		allDynamicSucceeded := true
		detailedStatuses := map[string]interface{}{
			"staticValidations":  map[string][]operator.StaticValidationResourceResult{},
			"dynamicValidations": map[string][]operator.DynamicValidationResourceResult{},
		}

		for _, svUnstructured := range svListUnstructured.Items {
			var sv operator.StaticValidation
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(svUnstructured.Object, &sv)
			if err != nil {
				log.Fatalf("Error converting StaticValidation to StaticValidation: %v", err)
			}

			staticStatuses := detailedStatuses["staticValidations"].(map[string][]operator.StaticValidationResourceResult)
			staticStatuses[svUnstructured.GetName()] = sv.Status.Resources

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

					dynamicStatuses := detailedStatuses["dynamicValidations"].(map[string][]operator.DynamicValidationResourceResult)
					dynamicStatuses[dvUnstructured.GetName()] = dv.Status.Resources

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
		} else if allStaticSucceeded {
			fmt.Println("\n✅ All StaticValidations succeeded!")
			fmt.Println("❌ Some DynamicValidations did not succeed or encountered errors.")
			err = api.SendSubmit(challenge.Id, true, false, detailedStatuses)
		} else {
			fmt.Println("\n❌ Some StaticValidations did not succeed or encountered errors.")
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
