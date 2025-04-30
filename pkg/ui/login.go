package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/constants"
	"github.com/zalando/go-keyring"
)

// LoginModel represents the model for the login process
type LoginModel struct {
	ApiKey       string
	ShowingInput bool
	Error        error
	Success      bool
	Quitting     bool
}

// NewLoginModel creates a new login model
func NewLoginModel() LoginModel {
	return LoginModel{
		ShowingInput: true,
	}
}

// Init initializes the model
func (m LoginModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (m LoginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.Quitting = true
			return m, tea.Quit
		}

	case error:
		m.Error = msg
		return m, tea.Quit

	case string:
		// Message from the API key input
		m.ApiKey = msg
		m.ShowingInput = false

		// Store the API key
		return m, func() tea.Msg {
			err := keyring.Set(constants.KeyringServiceName, "api_key", m.ApiKey)
			if err != nil {
				return err
			}
			m.Success = true
			return nil
		}
	}

	return m, nil
}

// View renders the current state
func (m LoginModel) View() string {
	if m.Quitting {
		return "Operation cancelled.\n"
	}

	var s strings.Builder

	s.WriteString(TitleStyle.Render("🔐 Login to Kubeasy"))
	s.WriteString("\n\n")

	if m.ShowingInput {
		s.WriteString(InfoStyle.Render("Please enter your API key to login."))
		s.WriteString("\n")
		s.WriteString(InfoStyle.Render("If you don't have an API key or forgot it, please visit https://kubeasy.dev/profile"))
		s.WriteString("\n\n")
		return s.String()
	}

	if m.Error != nil {
		s.WriteString(ErrorStyle.Render(fmt.Sprintf("❌ Error storing API key: %v", m.Error)))
		s.WriteString("\n")
		return s.String()
	}

	if m.Success {
		s.WriteString(SuccessStyle.Render("✅ API key successfully stored!"))
		s.WriteString("\n")
		s.WriteString(InfoStyle.Render("You can now use Kubeasy commands."))
		s.WriteString("\n")
	}

	return s.String()
}
