package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/argocd"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/kube"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/logger"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/ui"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/kind/pkg/cluster"
)

var replaceFlag bool

// Phase constants
const (
	phaseChecking           = "checking"
	phaseCheckingComponents = "checking-components"
	phaseCheckingArgoCD     = "checking-argocd" // New phase
	phaseDeleting           = "deleting"
	phaseCreating           = "creating"
	phaseInstallingArgoCD   = "installing-argocd"
	phaseWaitingAppsReady   = "waiting-apps-ready"
)

// Simple model for setup process
type setupModel struct {
	spinner        spinner.Model
	status         string
	err            error
	done           bool
	usingForce     bool
	currentPhase   string
	width          int
	isCheckOnlyRun bool // Flag to indicate if this is just a check run
}

// Message types
type setupDoneMsg struct{}
type setupErrMsg struct{ err error }
type setupPhaseMsg struct{ phase string }

func initialModel() setupModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return setupModel{
		spinner:        s,
		status:         "Initializing setup...", // Initial status
		usingForce:     replaceFlag,
		isCheckOnlyRun: false, // Initialize the flag
	}
}

func (m setupModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		// Start directly with checking phase
		func() tea.Msg {
			return setupPhaseMsg{phase: phaseChecking}
		},
	)
}

func (m setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg: // Add case for WindowSizeMsg
		m.width = msg.Width
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case setupDoneMsg:
		m.done = true
		return m, tea.Quit
	case setupErrMsg:
		m.err = msg.err
		m.done = true
		return m, tea.Quit
	case setupPhaseMsg:
		m.currentPhase = msg.phase
		switch msg.phase {
		case phaseChecking:
			m.status = "Checking cluster configuration..."
			return m, checkClusterExistence
		case phaseCheckingComponents:
			m.isCheckOnlyRun = true
			m.status = "Cluster exists, checking components..."
			// Transition to checking ArgoCD installation
			return m, func() tea.Msg { return setupPhaseMsg{phase: phaseCheckingArgoCD} }
		case phaseCheckingArgoCD: // Handle the new phase
			m.status = "Checking core component installation status..."
			return m, checkArgoCDInstallation
		case phaseDeleting:
			m.status = "Removing existing cluster (--force)..."
			return m, deleteCluster
		case phaseCreating:
			m.status = "Creating Kubernetes cluster..."
			return m, createCluster
		case phaseInstallingArgoCD:
			m.status = "Installing core components on the cluster..."
			return m, installArgoCD
		case phaseWaitingAppsReady:
			m.status = "Waiting for core applications to be ready..."
			return m, waitForArgoCDAppsReady
		}
	}
	return m, nil
}

func (m setupModel) View() string {
	if m.err != nil {
		// Use WordWrap for potentially long error messages
		// Use backticks for multi-line string or escape newline characters
		// Emojis are fine in Go strings
		errorText := fmt.Sprintf("\n❌ Error: %v\n", m.err)
		return lipgloss.NewStyle().Width(m.width).Render(errorText)
	}
	if m.done {
		var successMsg string
		if m.isCheckOnlyRun {
			// Use backticks for multi-line string
			successMsg = `
✅ Cluster and applications are already set up correctly.
`
		} else {
			// Use backticks for multi-line string
			successMsg = `
✅ Cluster successfully created!
`
			successMsg += ui.InfoStyle.Render("You can now start Kubeasy challenges.\n")
		}
		return ui.SuccessStyle.Render(successMsg)
	}
	// Use backticks for multi-line string
	statusText := fmt.Sprintf(`
 %s %s
`, m.spinner.View(), m.status)
	return statusText
}

// Command to check if cluster exists using kind provider
func checkClusterExistence() tea.Msg {
	// Use package-level logger functions
	logger.Info("Checking for existing 'kubeasy' kind cluster")
	provider := cluster.NewProvider()
	clusters, err := provider.List()
	if err != nil {
		logger.Error("Error listing kind clusters: %v", err)
		return setupErrMsg{err: fmt.Errorf("error checking kind clusters: %w", err)}
	}

	clusterExists := false
	for _, c := range clusters {
		if c == "kubeasy" {
			clusterExists = true
			break
		}
	}

	if clusterExists && replaceFlag {
		logger.Info("Existing 'kubeasy' cluster found and --force is set. Proceeding to delete.")
		return setupPhaseMsg{phase: phaseDeleting}
	} else if clusterExists {
		// Cluster exists, but --force is not used. Proceed to check components.
		logger.Info("Cluster 'kubeasy' already exists. Checking components...")
		return setupPhaseMsg{phase: phaseCheckingComponents} // Go to checking phase
	}

	logger.Info("No existing 'kubeasy' cluster found. Proceeding to create.")
	return setupPhaseMsg{phase: phaseCreating}
}

// Command to check if ArgoCD seems installed
func checkArgoCDInstallation() tea.Msg {
	logger.Info("Checking if ArgoCD components are installed...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Short timeout for checks
	defer cancel()

	clientset, err := kube.GetKubernetesClient()
	if err != nil {
		logger.Error("Failed to get Kubernetes clientset for check: %v", err)
		return setupErrMsg{err: fmt.Errorf("failed to get Kubernetes client: %w", err)}
	}

	// 1. Check if argocd namespace exists
	_, err = clientset.CoreV1().Namespaces().Get(ctx, argocd.ArgoCDNamespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("ArgoCD namespace '%s' not found. Proceeding with installation.", argocd.ArgoCDNamespace)
			return setupPhaseMsg{phase: phaseInstallingArgoCD}
		}
		logger.Error("Error checking ArgoCD namespace '%s': %v", argocd.ArgoCDNamespace, err)
		return setupErrMsg{err: fmt.Errorf("error checking ArgoCD namespace: %w", err)}
	}
	logger.Debug("ArgoCD namespace '%s' found.", argocd.ArgoCDNamespace)

	// 2. Check for a key deployment (e.g., argocd-server)
	_, err = clientset.AppsV1().Deployments(argocd.ArgoCDNamespace).Get(ctx, "argocd-server", metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("ArgoCD deployment 'argocd-server' not found. Proceeding with installation.")
			return setupPhaseMsg{phase: phaseInstallingArgoCD}
		}
		logger.Error("Error checking ArgoCD deployment 'argocd-server': %v", err)
		return setupErrMsg{err: fmt.Errorf("error checking ArgoCD deployment: %w", err)}
	}
	logger.Debug("ArgoCD deployment 'argocd-server' found.")

	// 3. Check for a key statefulset (e.g., argocd-application-controller)
	_, err = clientset.AppsV1().StatefulSets(argocd.ArgoCDNamespace).Get(ctx, "argocd-application-controller", metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("ArgoCD statefulset 'argocd-application-controller' not found. Proceeding with installation.")
			return setupPhaseMsg{phase: phaseInstallingArgoCD}
		}
		logger.Error("Error checking ArgoCD statefulset 'argocd-application-controller': %v", err)
		return setupErrMsg{err: fmt.Errorf("error checking ArgoCD statefulset: %w", err)}
	}
	logger.Debug("ArgoCD statefulset 'argocd-application-controller' found.")

	// If all checks pass, assume ArgoCD is installed and proceed to check app readiness
	logger.Info("ArgoCD components seem to be installed. Proceeding to check application readiness.")
	return setupPhaseMsg{phase: phaseWaitingAppsReady}
}

// Command to delete existing cluster using kind provider
func deleteCluster() tea.Msg {
	logger.Info("Deleting existing 'kubeasy' kind cluster")
	provider := cluster.NewProvider()
	err := provider.Delete("kubeasy", "") // Pass kubeconfig path if needed, empty uses default
	if err != nil {
		logger.Error("Error deleting kind cluster 'kubeasy': %v", err)
		return setupErrMsg{err: fmt.Errorf("error deleting kind cluster 'kubeasy': %w", err)}
	}

	logger.Info("Existing 'kubeasy' cluster deleted successfully")
	time.Sleep(1 * time.Second) // Give UI time to update
	return setupPhaseMsg{phase: phaseCreating}
}

// Command to create cluster using kind provider
func createCluster() tea.Msg {
	logger.Info("Creating 'kubeasy' kind cluster")
	provider := cluster.NewProvider()

	// Note: The check for existence and deletion logic is now primarily in checkClusterExistence and deleteCluster.
	// CreateCluster in kind doesn't have a built-in force flag like the previous custom implementation.
	// We rely on the preceding steps to handle deletion if --force was specified.

	// Remove the CreateWithNodeImage("") option to use the default Kind node image.
	err := provider.Create("kubeasy", cluster.CreateWithWaitForReady(5*time.Minute)) // Add other options as needed
	if err != nil {
		logger.Error("Error creating kind cluster 'kubeasy': %v", err)
		// Provide a more specific error message if possible
		return setupErrMsg{err: fmt.Errorf("failed to create kind cluster 'kubeasy': %w", err)}
	}

	logger.Info("Kind cluster 'kubeasy' created successfully")
	return setupPhaseMsg{phase: phaseInstallingArgoCD} // Proceed to ArgoCD installation
}

// Command to install ArgoCD on the cluster
func installArgoCD() tea.Msg {
	logger.Info("Installing ArgoCD")

	// Use the new ArgoCD package for installation
	options := argocd.DefaultInstallOptions()

	err := argocd.InstallArgoCD(options)
	if err != nil {
		logger.Error("Error installing ArgoCD: %v", err)
		return setupErrMsg{err: fmt.Errorf("error installing ArgoCD: %w", err)}
	}

	logger.Info("ArgoCD installed successfully")
	return setupPhaseMsg{phase: phaseWaitingAppsReady}
}

// Command to wait for ArgoCD applications to be ready
func waitForArgoCDAppsReady() tea.Msg {
	logger.Info("Waiting for ArgoCD applications to be ready")
	apps := []string{"kubeasy-cli-setup", "kyverno", "argocd", "kubeasy-challenge-operator"}
	err := argocd.WaitForArgoCDAppsReadyCore(apps, 8*time.Minute)
	if err != nil {
		logger.Error("Error waiting for ArgoCD apps: %v", err)
		return setupErrMsg{err: fmt.Errorf("error waiting for ArgoCD applications: %w", err)}
	}
	logger.Info("All ArgoCD applications are ready!")
	return setupDoneMsg{}
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup",
	Long:  "It will setup a local cluster for the Kubeasy challenges and install ArgoCD",
	Run: func(cmd *cobra.Command, args []string) {
		model := initialModel()

		p := tea.NewProgram(model)
		if _, err := p.Run(); err != nil {
			// Log the final error before exiting
			logger.Error("Setup failed: %v", err)
			// Ensure UI is inactive before printing final error to stderr
			fmt.Fprintf(os.Stderr, "Error during setup: %v\n", err)
			os.Exit(1)
		}
		// Success message is handled by the Bubble Tea view
	},
}

func init() {
	setupCmd.Flags().BoolVarP(&replaceFlag, "force", "f", false, "Force replacement of existing cluster")
	rootCmd.AddCommand(setupCmd)
}
