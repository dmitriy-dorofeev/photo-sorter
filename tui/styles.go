package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			PaddingLeft(2).
			PaddingRight(2)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A0A0A0"))

	highlightStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF4672"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))

	settingLabelStyle = lipgloss.NewStyle().
				Width(28)

	templateLabelStyle = lipgloss.NewStyle().
				Width(16)
)
