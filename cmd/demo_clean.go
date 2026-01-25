package cmd

import (
	"github.com/spf13/cobra"

	"github.com/kubeasy-dev/kubeasy-cli/internal/demo"
	"github.com/kubeasy-dev/kubeasy-cli/internal/kube"
	"github.com/kubeasy-dev/kubeasy-cli/internal/ui"
)

var demoCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean up demo resources",
	Long:  "Remove the demo namespace and stored token",
	RunE:  runDemoClean,
}

func init() {
	demoCmd.AddCommand(demoCleanCmd)
}

func runDemoClean(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	ui.Section("Cleaning Demo Resources")

	// Delete namespace
	clientset, err := kube.GetKubernetesClient()
	if err == nil {
		ui.Info("Deleting demo namespace...")
		if err := kube.DeleteNamespace(ctx, clientset, demo.DemoNamespace); err != nil {
			ui.Warning("Could not delete namespace: " + err.Error())
		} else {
			ui.Success("Namespace deleted!")
		}
	}

	// Delete stored token
	ui.Info("Removing stored token...")
	if err := demo.DeleteToken(); err != nil {
		ui.Warning("Could not remove token: " + err.Error())
	} else {
		ui.Success("Token removed!")
	}

	ui.Println()
	ui.Success("Demo cleanup complete!")

	return nil
}
