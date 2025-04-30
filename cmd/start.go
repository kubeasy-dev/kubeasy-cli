package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/api"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/argocd"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/kube"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/logger"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/ui" // Import the ui package for styles
	"github.com/spf13/cobra"
	"k8s.io/client-go/dynamic"
)

// Phase constants for start command
const (
	phaseStartFetchingChallenge = "fetching-challenge"
	phaseStartGettingKubeClient = "getting-kube-client" // Although quick, good to represent
	phaseStartCreatingArgoApp   = "creating-argo-app"
	phaseStartWaitingArgoApp    = "waiting-argo-app"
	phaseStartMarkingStarted    = "marking-started"
	phaseStartSettingNamespace  = "setting-namespace"
)

// Message types for start command
type startDoneMsg struct{}
type startErrMsg struct{ err error }
type startPhaseMsg struct{ phase string }

// Model for start command UI
type startModel struct {
	spinner       spinner.Model
	status        string
	err           error
	done          bool
	currentPhase  string
	width         int
	challengeSlug string
	challenge     *api.ChallengeEntity // Store fetched challenge details
	dynamicClient dynamic.Interface
}

func initialStartModel(slug string) startModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return startModel{
		spinner:       s,
		status:        "Initializing challenge start...",
		challengeSlug: slug,
	}
}

func (m startModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg { return startPhaseMsg{phase: phaseStartFetchingChallenge} },
	)
}

func (m startModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case startDoneMsg:
		m.done = true
		return m, tea.Quit
	case startErrMsg:
		m.err = msg.err
		m.done = true
		return m, tea.Quit
	case startPhaseMsg:
		m.currentPhase = msg.phase
		switch msg.phase {
		case phaseStartFetchingChallenge:
			m.status = fmt.Sprintf("Fetching details for challenge '%s'...", m.challengeSlug)
			return m, fetchChallengeDetails(m.challengeSlug)
		case phaseStartGettingKubeClient:
			m.status = "Connecting to Kubernetes cluster..."
			return m, getKubeClient
		case phaseStartCreatingArgoApp:
			m.status = fmt.Sprintf("Creating/Updating Argo CD application for '%s'...", m.challengeSlug)
			return m, createArgoApp(m.dynamicClient, m.challenge.Slug) // Pass client and challenge
		case phaseStartWaitingArgoApp:
			m.status = "Waiting for challenge resources to be ready..."
			return m, waitForArgoApp(m.challengeSlug)
		case phaseStartMarkingStarted:
			m.status = fmt.Sprintf("Marking challenge '%s' as started...", m.challengeSlug)
			return m, markChallengeStarted(m.challengeSlug)
		case phaseStartSettingNamespace:
			m.status = fmt.Sprintf("Switching Kubernetes context namespace to '%s'/%s'...", "kind-kubeasy", m.challengeSlug)
			return m, setKubeNamespace(m.challengeSlug)
		}
	// Internal messages to pass data between phases
	case *api.ChallengeEntity: // Message carrying fetched challenge
		m.challenge = msg
		return m, func() tea.Msg { return startPhaseMsg{phase: phaseStartGettingKubeClient} }
	case dynamic.Interface: // Message carrying dynamic client
		m.dynamicClient = msg
		return m, func() tea.Msg { return startPhaseMsg{phase: phaseStartCreatingArgoApp} }
	}
	return m, nil
}

func (m startModel) View() string {
	if m.err != nil {
		errorText := fmt.Sprintf("\n❌ Error starting challenge '%s': %v\n", m.challengeSlug, m.err)
		return lipgloss.NewStyle().Width(m.width).Render(errorText)
	}
	if m.done {
		successMsg := fmt.Sprintf("\n✅ Challenge '%s' started successfully!\n", m.challengeSlug)
		successMsg += ui.InfoStyle.Render(fmt.Sprintf("Kubernetes context namespace set to '%s/%s'.\n", "kind-kubeasy", m.challengeSlug))
		return ui.SuccessStyle.Render(successMsg)
	}
	statusText := fmt.Sprintf("\n %s %s\n", m.spinner.View(), m.status)
	return statusText
}

// --- Phase Functions ---

func fetchChallengeDetails(slug string) tea.Cmd {
	return func() tea.Msg {
		logger.Debug("Fetching challenge details for '%s'", slug)
		challenge, err := api.GetChallenge(slug)
		if err != nil {
			logger.Error("Failed to get challenge '%s': %v", slug, err)
			return startErrMsg{err: fmt.Errorf("fetching challenge details failed: %w", err)}
		}
		logger.Debug("Challenge details fetched successfully for '%s'", slug)
		return challenge // Send the fetched challenge back to the model
	}
}

func getKubeClient() tea.Msg {
	logger.Debug("Getting Kubernetes dynamic client...")
	dynamicClient, err := kube.GetDynamicClient()
	if err != nil {
		logger.Error("Failed to get Kubernetes dynamic client: %v", err)
		return startErrMsg{err: fmt.Errorf("connecting to Kubernetes failed: %w", err)}
	}
	logger.Debug("Kubernetes dynamic client obtained.")
	return dynamicClient // Send the client back to the model
}

func createArgoApp(dynamicClient dynamic.Interface, challengeSlug string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		logger.Debug("Creating/Updating Argo CD Application '%s' in namespace '%s'", challengeSlug, argocd.ArgoCDNamespace)
		err := argocd.CreateOrUpdateChallengeApplication(ctx, dynamicClient, challengeSlug)
		if err != nil {
			logger.Error("Error managing Argo CD Application for challenge '%s': %v", challengeSlug, err)
			return startErrMsg{err: fmt.Errorf("managing Argo CD Application failed: %w", err)}
		}
		logger.Info("Argo CD Application '%s' created/updated.", challengeSlug)
		return startPhaseMsg{phase: phaseStartWaitingArgoApp}
	}
}

func waitForArgoApp(challengeSlug string) tea.Cmd {
	return func() tea.Msg {
		logger.Info("Waiting for Argo CD application '%s' to become ready...", challengeSlug)
		// Use a reasonable timeout
		err := argocd.WaitForArgoCDAppsReadyCore([]string{challengeSlug}, 5*time.Minute)
		if err != nil {
			logger.Error("Failed waiting for Argo CD application '%s': %v", challengeSlug, err)
			return startErrMsg{err: fmt.Errorf("waiting for challenge resources failed: %w", err)}
		}
		logger.Info("Argo CD application '%s' is ready.", challengeSlug)
		return startPhaseMsg{phase: phaseStartMarkingStarted}
	}
}

func markChallengeStarted(challengeSlug string) tea.Cmd {
	return func() tea.Msg {
		logger.Debug("Marking challenge '%s' as started in API", challengeSlug)
		err := api.StartChallenge(challengeSlug)
		if err != nil {
			logger.Error("Failed to mark challenge '%s' as started in API: %v", challengeSlug, err)
			// Consider if this should be a fatal error for the command
			return startErrMsg{err: fmt.Errorf("marking challenge as started failed: %w", err)}
		}
		logger.Info("Challenge '%s' marked as started.", challengeSlug)
		return startPhaseMsg{phase: phaseStartSettingNamespace}
	}
}

func setKubeNamespace(challengeSlug string) tea.Cmd {
	return func() tea.Msg {
		kubeContextName := "kind-kubeasy" // Target context name - make configurable?
		targetNamespace := challengeSlug
		logger.Info("Setting Kubernetes context '%s' namespace to '%s'...", kubeContextName, targetNamespace)
		err := kube.SetNamespaceForContext(kubeContextName, targetNamespace)
		if err != nil {
			// Log the error but don't fail the whole command, maybe just warn
			logger.Error("Failed to set namespace '%s' for context '%s': %v", targetNamespace, kubeContextName, err)
			// Optionally return a specific warning message type or just log and proceed to done
			// For now, we treat it as success but log the error.
			fmt.Println(ui.ErrorStyle.Render(fmt.Sprintf("Warning: Could not set Kubernetes context namespace: %v", err)))
		} else {
			logger.Info("Successfully set Kubernetes context '%s' namespace to '%s'.", kubeContextName, targetNamespace)
		}
		return startDoneMsg{} // Signal completion
	}
}

// --- Cobra Command ---

var startChallengeCmd = &cobra.Command{
	Use:   "start [challenge-slug]",
	Short: "Start a challenge",
	Long:  `Starts a challenge by installing the necessary components into the local Kubernetes cluster.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		challengeSlug := args[0]
		logger.Info("Initiating start for challenge: %s", challengeSlug)

		model := initialStartModel(challengeSlug)
		p := tea.NewProgram(model)

		finalModel, err := p.Run()
		if err != nil {
			logger.Error("Error during challenge start: %v", err)
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Check the final model state for errors that occurred within the Bubble Tea app
		if finalStartModel, ok := finalModel.(startModel); ok && finalStartModel.err != nil {
			// Error is already logged by the phase function and displayed by the View
			os.Exit(1)
		}

		// Success message is handled by the Bubble Tea view
		logger.Info("Challenge '%s' start process completed.", challengeSlug)
	},
}

func init() {
	challengeCmd.AddCommand(startChallengeCmd)
}
