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

// Structure pour stocker les détails de validation enrichis
type detailedValidationInfo struct {
	Name    string                      `json:"name"`
	Phase   string                      `json:"phase"`
	Message string                      `json:"message,omitempty"`
	Results []operator.ValidationResult `json:"results,omitempty"`
}

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

		// --- Define ValidationRequest GVR ---
		vrGVR := schema.GroupVersionResource{
			Group:    "challenge.kubeasy.dev", // Correspond à l'API importée
			Version:  "v1alpha1",              // Correspond à l'API importée
			Resource: "validationrequests",    // Correspond à l'API importée
		}

		// --- Determine Namespace ---
		namespace := challengeSlug

		// --- List ValidationRequests ---
		vrListUnstructured, err := dynamicClient.Resource(vrGVR).Namespace(namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			log.Fatalf("Error listing ValidationRequests in namespace %s: %v", namespace, err)
		}

		if len(vrListUnstructured.Items) == 0 {
			fmt.Printf("No ValidationRequests found in namespace %s. Cannot verify submission.\n", namespace)
			return // Ou peut-être que c'est une réussite ? À définir.
		}

		// --- Check Status ---
		allSucceeded := true
		// Utiliser la nouvelle structure pour stocker les détails
		detailedStatuses := []detailedValidationInfo{}

		for _, vrUnstructured := range vrListUnstructured.Items {
			var vr operator.ValidationRequest
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(vrUnstructured.Object, &vr)
			// Convertir phase en string pour l'affichage et le stockage
			phaseStr := string(vr.Status.Phase)

			if err != nil {
				log.Printf("Error converting unstructured ValidationRequest %s/%s: %v", vrUnstructured.GetNamespace(), vrUnstructured.GetName(), err)
				allSucceeded = false
				// Ajouter un statut d'erreur si nécessaire, avec des infos limitées
				detailedStatuses = append(detailedStatuses, detailedValidationInfo{
					Name:  vrUnstructured.GetName(),
					Phase: "ConversionError", // Indiquer l'erreur de conversion
				})
				continue
			}

			fmt.Printf("  - ValidationRequest %s: Phase = %s\n", vr.GetName(), phaseStr)
			// Ajouter les informations détaillées à la slice
			detailedStatuses = append(detailedStatuses, detailedValidationInfo{
				Name:    vr.GetName(),
				Phase:   phaseStr,
				Message: vr.Status.Message,
				Results: vr.Status.Results, // Inclure les résultats
			})

			// Convertir phase en string pour les comparaisons
			if phaseStr == "" {
				log.Printf("status.phase is empty for %s/%s", vr.GetNamespace(), vr.GetName())
				allSucceeded = false
			} else if phaseStr != string(operator.ValidationSucceeded) { // Comparer avec la constante définie
				allSucceeded = false
			}
		}

		// --- Prepare Payload Map (pas de marshaling ici) ---
		payloadMap := map[string][]detailedValidationInfo{
			"validationRequests": detailedStatuses,
		}
		// NOTE: Le marshaling JSON est retiré d'ici.

		// --- Report Result & Call API ---
		if allSucceeded {
			fmt.Println("\n✅ All ValidationRequests succeeded!")
			err = api.SendSubmit(challenge.Id, true, true, payloadMap) // Envoyer la map Go
		} else {
			fmt.Println("\n❌ Some ValidationRequests did not succeed or encountered errors.")
			err = api.SendSubmit(challenge.Id, false, false, payloadMap) // Envoyer la map Go
		}

		if err != nil {
			log.Printf("Error sending submission: %v", err)
			// os.Exit(1) // Envisager de sortir avec une erreur
		}

		if !allSucceeded {
			os.Exit(1)
		}
	},
}

func init() {
	challengeCmd.AddCommand(submitCmd)
}
