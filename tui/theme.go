package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Theme содержит стили оформления для TUI.
type Theme struct {
	Title         lipgloss.Style
	Subtitle      lipgloss.Style
	Highlight     lipgloss.Style
	Success       lipgloss.Style
	Error         lipgloss.Style
	Help          lipgloss.Style
	SettingLabel  lipgloss.Style
	TemplateLabel lipgloss.Style
}

// NewLightTheme возвращает светлую цветовую схему.
func NewLightTheme() *Theme {
	return &Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#5B3CC4")).
			PaddingLeft(2).
			PaddingRight(2),
		Subtitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555")),
		Highlight: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5B3CC4")),
		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#008F5B")),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D90429")),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#777777")),
		SettingLabel: lipgloss.NewStyle().
			Width(28),
		TemplateLabel: lipgloss.NewStyle().
			Width(16),
	}
}

// NewDarkTheme возвращает тёмную цветовую схему.
func NewDarkTheme() *Theme {
	return &Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			PaddingLeft(2).
			PaddingRight(2),
		Subtitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A0A0A0")),
		Highlight: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A78BFA")),
		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF4672")),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")),
		SettingLabel: lipgloss.NewStyle().
			Width(28),
		TemplateLabel: lipgloss.NewStyle().
			Width(16),
	}
}

// ThemeMode описывает режим темы.
type ThemeMode int

const (
	ThemeModeAuto ThemeMode = iota
	ThemeModeLight
	ThemeModeDark
)

// themeMsg передаёт результат детекции системной темы.
type themeMsg struct {
	mode ThemeMode
}

// detectThemeCmd определяет предпочтительную тему (async).
func detectThemeCmd() tea.Cmd {
	return func() tea.Msg {
		return themeMsg{mode: systemThemeMode()}
	}
}
