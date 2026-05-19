package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// sourcesModel — состояние экрана выбора источника.
type sourcesModel struct {
	dirBrowserModel
	selected map[string]struct{} // множество выбранных путей
}

func newSourcesModel() sourcesModel {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}

	return sourcesModel{
		dirBrowserModel: newDirBrowserModel(home),
		selected:        make(map[string]struct{}),
	}
}

func (s sourcesModel) Init() tea.Cmd {
	return nil
}

func (m Model) updateSources(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
			if len(m.Sources) > 0 {
				m.screen = ScreenTarget
				m.target.currentDir = m.sources.currentDir
				items, err := loadDirItems(m.target.currentDir)
				m.target.items = items
				if err != nil {
					m.target.readErr = err.Error()
				} else {
					m.target.readErr = ""
				}
				m.target.cursor = 0
			}
			return m, nil
		default:
		}

		switch msg.String() {
		case " ": // пробел — добавить/удалить папку из списка источников
			if len(m.sources.items) == 0 {
				return m, nil
			}
			item := m.sources.items[m.sources.cursor]
			if item.isParent {
				return m, nil // нельзя выбрать ".."
			}
			path := filepath.Clean(item.path)
			if _, ok := m.sources.selected[path]; ok {
				delete(m.sources.selected, path)
			} else {
				m.sources.selected[path] = struct{}{}
			}
			// Синхронизируем Model.Sources
			m.Sources = make([]string, 0, len(m.sources.selected))
			for p := range m.sources.selected {
				m.Sources = append(m.Sources, p)
			}
			sort.Strings(m.Sources)
			return m, nil

		}
	}

	return m, nil
}

func (m Model) viewSources() string {
	var b strings.Builder

	b.WriteString(m.theme.Title.Render(" photo-sorter "))
	b.WriteString("\n\n")
	b.WriteString(m.theme.Subtitle.Render("Шаг 1. Выбор источника"))
	b.WriteString("\n")
	if notice := updateNotice(m); notice != "" {
		b.WriteString(notice)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// ── Блок выбора ──
	b.WriteString(m.theme.Highlight.Render("Текущая папка: "))
	b.WriteString(m.sources.currentDir)
	b.WriteString("\n\n")

	// Список папок
	if m.sources.readErr != "" {
		b.WriteString(m.theme.Error.Render("  Ошибка чтения: "+m.sources.readErr) + "\n")
	} else {
		for i, item := range m.sources.items {
			cursor := "  "
			if m.sources.cursor == i {
				cursor = m.theme.Highlight.Render("▸ ")
			}

			icon := "📁"
			if item.isParent {
				icon = "⬆️"
			}

			check := "  "
			if _, ok := m.sources.selected[filepath.Clean(item.path)]; ok {
				check = m.theme.Success.Render("✓ ")
			}

			b.WriteString(fmt.Sprintf("%s%s%s %s\n", cursor, check, icon, item.name))
		}

		if len(m.sources.items) == 0 {
			b.WriteString(m.theme.Error.Render("  (папка пуста)\n"))
		}
	}

	b.WriteString("\n")

	// ── Блок выбранных источников ──
	b.WriteString(m.theme.Highlight.Render("Источники: "))
	if len(m.Sources) == 0 {
		b.WriteString("(не выбрано)\n")
	} else {
		for i, src := range m.Sources {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(src)
		}
		b.WriteString(fmt.Sprintf(" (%d)\n", len(m.Sources)))
	}

	b.WriteString("\n")

	nextHint := m.theme.Help.Render("→ — продолжить »")
	if len(m.Sources) == 0 {
		nextHint = m.theme.Help.Render("→ — продолжить (выберите хотя бы один источник)")
	}

	b.WriteString(m.theme.Help.Render(
		"↑/↓ — выбрать • enter — открыть • backspace — вверх • пробел — выбрать/убрать • → — продолжить • esc — выход",
	))
	b.WriteString("\n")
	b.WriteString(nextHint)

	return b.String()
}
