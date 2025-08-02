package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/kubeasy-dev/kubeasy-cli/pkg/argocd"
	"github.com/spf13/cobra"
	"sigs.k8s.io/kind/pkg/cluster"
)

var replaceFlag bool

func checkClusterExists() bool {
	provider := cluster.NewProvider()
	clusters, err := provider.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing clusters: %v\n", err)
		os.Exit(1)
	}
	clusterExists := false
	for _, c := range clusters {
		if c == "kubeasy" {
			clusterExists = true
			break
		}
	}
	return clusterExists
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup",
	Long:  "It will setup a local cluster for the Kubeasy challenges and install ArgoCD",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("[Kubeasy] Checking if cluster exists...")
		if !checkClusterExists() {
			fmt.Println("Creating kind cluster 'kubeasy'...")
			if err := cluster.NewProvider().Create("kubeasy"); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create kind cluster 'kubeasy': %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Kind cluster 'kubeasy' created successfully.")
		}

		fmt.Println("Checking if ArgoCD is already installed...")
		isInstalled, err := argocd.IsArgoCDInstalled()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking ArgoCD installation status: %v\n", err)
			os.Exit(1)
		}

		if isInstalled {
			fmt.Println("ArgoCD is already installed and ready.")
		} else {
			fmt.Println("Installing ArgoCD...")
			options := argocd.DefaultInstallOptions()
			if err := argocd.InstallArgoCD(options); err != nil {
				fmt.Fprintf(os.Stderr, "Error installing ArgoCD: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("ArgoCD installed successfully.")
		}

		fmt.Println("Waiting for ArgoCD applications to be ready...")
		apps := []string{"kubeasy-cli-setup", "kyverno", "argocd", "kubeasy-challenge-operator"}
		if err := argocd.WaitForArgoCDAppsReadyCore(apps, 8*time.Minute); err != nil {
			fmt.Fprintf(os.Stderr, "Error waiting for ArgoCD apps: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("All ArgoCD applications are ready!")
	},
}

func init() {
	setupCmd.Flags().BoolVarP(&replaceFlag, "force", "f", false, "Force replacement of existing cluster")
	rootCmd.AddCommand(setupCmd)
}
