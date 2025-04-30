package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/kubeasy-dev/kubeasy-cli/pkg/api"
)

// ChallengeModel for displaying challenge details
type ChallengeModel struct {
	Challenge       *api.ChallengeEntity
	RenderedContent string
	Loading         bool
	Error           error
	Width           int
	Height          int
}

// NewChallengeModel creates a new challenge model
func NewChallengeModel() ChallengeModel {
	return ChallengeModel{
		Loading: true,
		Width:   80,
	}
}

// Init initializes the model
func (m ChallengeModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (m ChallengeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEsc || msg.String() == "q" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil
	case *api.ChallengeEntity:
		m.Challenge = msg
		m.Loading = false

		// Render markdown content
		return m, func() tea.Msg {
			// Render the initial situation
			out, err := glamour.Render(m.Challenge.InitialSituation, "dark")
			if err != nil {
				return err
			}
			out2, err := glamour.Render(m.Challenge.Objective, "dark")
			if err != nil {
				return err
			}
			return out + "\n\n" + out2
		}
	case string:
		// Message containing rendered markdown
		m.RenderedContent = msg
		return m, nil
	case error:
		m.Error = msg
		m.Loading = false
		return m, nil
	}

	return m, nil
}

// View renders the current state
func (m ChallengeModel) View() string {
	if m.Loading {
		return "Loading challenge...\n"
	}

	if m.Error != nil {
		return fmt.Sprintf("Error: %v\n", m.Error)
	}

	if m.Challenge == nil {
		return "Challenge not found.\n"
	}

	var s strings.Builder

	// Title
	s.WriteString(TitleStyle.Render(fmt.Sprintf("Challenge: %s", m.Challenge.Title)))
	s.WriteString("\n")

	// Challenge information
	diffStyle, ok := DifficultyStyles[strings.ToLower(m.Challenge.Difficulty)]
	if !ok {
		diffStyle = lipgloss.NewStyle()
	}

	s.WriteString(fmt.Sprintf("Difficulty: %s", diffStyle.Render(m.Challenge.Difficulty)))
	s.WriteString("   ")
	s.WriteString(fmt.Sprintf("Theme: %s", ThemeStyle.Render(m.Challenge.Theme)))
	s.WriteString("\n\n")

	// Description
	s.WriteString(InfoStyle.Render(m.Challenge.Description))
	s.WriteString("\n\n")

	// Main content
	if m.RenderedContent != "" {
		s.WriteString(m.RenderedContent)
	}

	s.WriteString("\n\nPress q to quit.\n")

	return s.String()
}
