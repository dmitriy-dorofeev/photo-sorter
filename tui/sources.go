package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// dirItem описывает один элемент в списке папок.
type dirItem struct {
	name     string
	path     string
	isParent bool // true для ".."
}

// sourcesModel — состояние экрана выбора источника.
type sourcesModel struct {
	currentDir string
	items      []dirItem
	cursor     int
	width      int
	height     int
}

func newSourcesModel() sourcesModel {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}

	return sourcesModel{
		currentDir: home,
		items:      loadDirItems(home),
		cursor:     0,
	}
}

func (s sourcesModel) Init() tea.Cmd {
	return nil
}

// loadDirItems возвращает список папок в директории + элемент "..".
func loadDirItems(dir string) []dirItem {
	var items []dirItem

	clean := filepath.Clean(dir)
	if clean != "/" {
		items = append(items, dirItem{
			name:     "..",
			path:     filepath.Dir(clean),
			isParent: true,
		})
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return items
	}

	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			items = append(items, dirItem{
				name: e.Name(),
				path: filepath.Join(dir, e.Name()),
			})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].isParent && !items[j].isParent {
			return true
		}
		if !items[i].isParent && items[j].isParent {
			return false
		}
		return items[i].name < items[j].name
	})

	return items
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
			if m.sources.cursor > 0 {
				m.sources.cursor--
			}
			return m, nil

		case tea.KeyDown:
			if m.sources.cursor < len(m.sources.items)-1 {
				m.sources.cursor++
			}
			return m, nil

		case tea.KeyEnter:
			if len(m.sources.items) == 0 {
				return m, nil
			}
			item := m.sources.items[m.sources.cursor]
			m.sources.currentDir = item.path
			m.sources.items = loadDirItems(item.path)
			m.sources.cursor = 0
			return m, nil

		case tea.KeyBackspace:
			parent := filepath.Dir(filepath.Clean(m.sources.currentDir))
			if parent != m.sources.currentDir {
				m.sources.currentDir = parent
				m.sources.items = loadDirItems(parent)
				m.sources.cursor = 0
			}
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
