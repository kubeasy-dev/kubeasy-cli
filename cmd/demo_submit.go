package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kubeasy-dev/kubeasy-cli/internal/api"
	"github.com/kubeasy-dev/kubeasy-cli/internal/demo"
	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
)

var demoSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Submit your demo solution",
	Long: `Submit your demo challenge solution for validation.

This command will:
1. Check if the nginx pod is running in the demo namespace
2. Send the results to the Kubeasy server
3. Display your validation status`,
	RunE: runDemoSubmit,
}

func init() {
	demoCmd.AddCommand(demoSubmitCmd)
}

func runDemoSubmit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	ui.Section("Submitting Demo Solution")

	// 1. Load token
	token, err := demo.LoadToken()
	if err != nil || token == "" {
		ui.Error("No demo token found. Run 'kubeasy demo start --token=xxx' first.")
		return fmt.Errorf("no demo token")
	}

	// 2. Get Kubernetes client
	clientset, err := kube.GetKubernetesClient()
	if err != nil {
		ui.Error("Failed to connect to cluster: " + err.Error())
		ui.Info("Make sure the cluster is running with 'kubeasy demo start'")
		return err
	}

	// 3. Run validations
	ui.Info("Running validations...")
	ui.Println()

	results := demo.ValidateDemo(ctx, clientset)

	// 4. Display results
	allPassed := true
	for _, result := range results {
		// Find objective metadata
		var title string
		for _, obj := range demo.DemoObjectives {
			if obj.Key == result.ObjectiveKey {
				title = obj.Title
				break
			}
		}

		if result.Passed {
			ui.Success(title)
		} else {
			ui.Error(title)
			allPassed = false
		}

		if result.Message != "" {
			ui.Info("  " + result.Message)
		}
	}

	ui.Println()

	// 5. Submit to backend
	ui.Info("Submitting results...")

	response, err := api.SendDemoSubmit(token, results)
	if err != nil {
		ui.Error("Failed to submit: " + err.Error())
		return err
	}

	ui.Println()

	// 6. Display final result
	if response.Success && allPassed {
		ui.Section("Congratulations!")
		ui.Success(response.Message)
		ui.Println()
		ui.Info("Ready to learn more? Sign up at https://kubeasy.dev/get-started")
		ui.Info("You'll get access to 30+ challenges and track your progress!")
	} else {
		ui.Section("Not Quite There Yet")
		ui.Warning(response.Message)
		ui.Println()
		ui.Info("Tips:")
		ui.Info("  1. Check if the pod exists: kubectl get pods -n demo")
		ui.Info("  2. Check pod status: kubectl describe pod nginx -n demo")
		ui.Info("  3. Wait for the pod to be Running before submitting")
	}

	return nil
}
