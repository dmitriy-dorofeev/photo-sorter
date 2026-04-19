package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// targetModel — состояние экрана выбора целевой папки.
type targetModel struct {
	currentDir string
	items      []dirItem
	cursor     int
	width      int
	height     int
}

func newTargetModel() targetModel {
	return targetModel{}
}

func (t targetModel) Init() tea.Cmd {
	return nil
}

func (m Model) updateTarget(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.target.width = msg.Width
		m.target.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyUp:
			if m.target.cursor > 0 {
				m.target.cursor--
			}
			return m, nil

		case tea.KeyDown:
			if m.target.cursor < len(m.target.items)-1 {
				m.target.cursor++
			}
			return m, nil

		case tea.KeyEnter, tea.KeyRight:
			if len(m.target.items) == 0 {
				return m, nil
			}
			item := m.target.items[m.target.cursor]
			m.target.currentDir = item.path
			m.target.items = loadDirItems(item.path)
			m.target.cursor = 0
			return m, nil

		case tea.KeyBackspace, tea.KeyLeft:
			parent := filepath.Dir(filepath.Clean(m.target.currentDir))
			if parent != m.target.currentDir {
				m.target.currentDir = parent
				m.target.items = loadDirItems(parent)
				m.target.cursor = 0
			}
			return m, nil
		}

		switch msg.String() {
		case " ", "t": // пробел или t — выбрать папку под курсором как цель
			if len(m.target.items) == 0 {
				return m, nil
			}
			item := m.target.items[m.target.cursor]
			if item.isParent {
				return m, nil // нельзя выбрать ".."
			}
			m.Target = filepath.Clean(item.path)
			return m, nil

		case "b": // назад к источникам
			m.screen = ScreenSources
			return m, nil

		case "n": // дальше — настройки
			if m.Target != "" {
				m.screen = ScreenSettings
			}
			return m, nil
		}
	}

	return m, nil
}

func (m Model) viewTarget() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(" photo-sorter "))
	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render("Шаг 2. Выбор целевой папки"))
	b.WriteString("\n\n")

	// ── Выбранный источник (read-only) ──
	b.WriteString(highlightStyle.Render("Источник: "))
	b.WriteString(m.Source + "\n")
	b.WriteString("\n")

	// ── Блок выбора цели ──
	b.WriteString(highlightStyle.Render("Текущая папка: "))
	b.WriteString(m.target.currentDir)
	b.WriteString("\n\n")

	// Список папок
	for i, item := range m.target.items {
		cursor := "  "
		if m.target.cursor == i {
			cursor = highlightStyle.Render("▸ ")
		}

		icon := "📁"
		if item.isParent {
			icon = "⬆️"
		}

		b.WriteString(fmt.Sprintf("%s%s %s\n", cursor, icon, item.name))
	}

	if len(m.target.items) == 0 {
		b.WriteString(errorStyle.Render("  (папка пуста)\n"))
	}

	b.WriteString("\n")

	// ── Выбранная цель ──
	b.WriteString(highlightStyle.Render("Цель: "))
	if m.Target == "" {
		b.WriteString("(не выбрано)\n")
	} else {
		b.WriteString(m.Target + "\n")
	}

	b.WriteString("\n")

	nextHint := helpStyle.Render("n — продолжить »")
	if m.Target == "" {
		nextHint = helpStyle.Render("n — продолжить (выберите целевую папку)")
	}

	b.WriteString(helpStyle.Render(
		"↑/↓ — выбрать • enter — открыть • ←/backspace — назад • пробел/t — выбрать цель • b — назад • esc — выход",
	))
	b.WriteString("\n")
	b.WriteString(nextHint)

	return b.String()
}
