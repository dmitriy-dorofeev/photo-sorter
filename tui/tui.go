// Package tui содержит консольный интерфейс на базе bubbletea.
package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// Run запускает TUI-приложение.
func Run() {
	p := tea.NewProgram(NewModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Ошибка запуска TUI: %v\n", err)
		os.Exit(1)
	}
}
