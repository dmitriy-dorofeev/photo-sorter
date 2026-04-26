package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// sourcesModel — состояние экрана выбора источника.
type sourcesModel struct {
	dirBrowserModel
}

func newSourcesModel() sourcesModel {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}

	return sourcesModel{
		dirBrowserModel: newDirBrowserModel(home),
	}
}

func (s sourcesModel) Init() tea.Cmd {
	return nil
}

func (m Model) updateSources(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case updateCheckMsg:
		m.updateResult = &msg.result
		return m, nil

	case tea.WindowSizeMsg:
		m.sources.width = msg.Width
		m.sources.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyUp:
			m.sources.moveUp()
			return m, nil

		case tea.KeyDown:
			m.sources.moveDown()
			return m, nil

		case tea.KeyEnter:
			m.sources.enter()
			return m, nil

		case tea.KeyBackspace:
			m.sources.goBack()
			return m, nil

		case tea.KeyRight:
			if m.Source != "" {
				m.screen = ScreenTarget
				m.target.currentDir = m.sources.currentDir
				m.target.items = loadDirItems(m.target.currentDir)
				m.target.cursor = 0
			}
			return m, nil
		}

		switch msg.String() {
		case " ": // пробел — выбрать/заменить папку под курсором как источник
			if len(m.sources.items) == 0 {
				return m, nil
			}
			item := m.sources.items[m.sources.cursor]
			if item.isParent {
				return m, nil // нельзя выбрать ".."
			}
			m.Source = filepath.Clean(item.path)
			return m, nil

		}
	}

	return m, nil
}

func (m Model) viewSources() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(" photo-sorter "))
	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render("Шаг 1. Выбор источника"))
	b.WriteString("\n")
	if notice := updateNotice(m); notice != "" {
		b.WriteString(notice)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// ── Блок выбора ──
	b.WriteString(highlightStyle.Render("Текущая папка: "))
	b.WriteString(m.sources.currentDir)
	b.WriteString("\n\n")

	// Список папок
	for i, item := range m.sources.items {
		cursor := "  "
		if m.sources.cursor == i {
			cursor = highlightStyle.Render("▸ ")
		}

		icon := "📁"
		if item.isParent {
			icon = "⬆️"
		}

		b.WriteString(fmt.Sprintf("%s%s %s\n", cursor, icon, item.name))
	}

	if len(m.sources.items) == 0 {
		b.WriteString(errorStyle.Render("  (папка пуста)\n"))
	}

	b.WriteString("\n")

	// ── Блок выбранного источника ──
	b.WriteString(highlightStyle.Render("Источник: "))
	if m.Source == "" {
		b.WriteString("(не выбрано)\n")
	} else {
		b.WriteString(m.Source + "\n")
	}

	b.WriteString("\n")

	nextHint := helpStyle.Render("→ — продолжить »")
	if m.Source == "" {
		nextHint = helpStyle.Render("→ — продолжить (выберите источник)")
	}

	b.WriteString(helpStyle.Render(
		"↑/↓ — выбрать • enter — открыть • backspace — вверх • пробел — выбрать источник • → — продолжить • esc — выход",
	))
	b.WriteString("\n")
	b.WriteString(nextHint)

	return b.String()
}
