package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Common styles for the UI components
var (
	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2ECC71")).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E74C3C")).
			Bold(true)

	InfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3498DB"))

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F39C12")).
			MarginTop(1).
			MarginBottom(1)

	ProgressStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F39C12"))

	DifficultyStyles = map[string]lipgloss.Style{
		"easy":   lipgloss.NewStyle().Background(lipgloss.Color("#2ECC71")).Padding(0, 1),
		"medium": lipgloss.NewStyle().Background(lipgloss.Color("#F39C12")).Padding(0, 1),
		"hard":   lipgloss.NewStyle().Background(lipgloss.Color("#E74C3C")).Padding(0, 1),
	}

	ThemeStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#9B59B6")).
			Padding(0, 1)
)
