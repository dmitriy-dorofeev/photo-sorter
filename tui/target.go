package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// targetModel — состояние экрана выбора целевой папки.
type targetModel struct {
	dirBrowserModel
	creating  bool
	input     textinput.Model
	createErr string
}

func newTargetModel() targetModel {
	ti := textinput.New()
	ti.Placeholder = "Новая папка"
	ti.CharLimit = 255
	ti.Width = 40

	return targetModel{
		input: ti,
	}
}

func (t targetModel) Init() tea.Cmd {
	return nil
}

func (m Model) updateTarget(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Режим создания новой папки
	if m.target.creating {
		return m.updateTargetCreating(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyUp:
			m.target.moveUp()
			return m, nil

		case tea.KeyDown:
			m.target.moveDown()
			return m, nil

		case tea.KeyEnter:
			m.target.enter()
			return m, nil

		case tea.KeyBackspace:
			m.target.goBack()
			return m, nil

		case tea.KeyRight:
			if m.Target != "" {
				m.screen = ScreenQuickStart
			}
			return m, nil

		case tea.KeyLeft:
			m.screen = ScreenSources
			return m, nil
		default:
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

		case "n": // n — создать новую папку
			m.target.creating = true
			m.target.createErr = ""
			m.target.input.SetValue("")
			m.target.input.Focus()
			return m, textinput.Blink

		case "c": // c — выбрать текущую папку как цель
			m.Target = filepath.Clean(m.target.currentDir)
			return m, nil
		}
	}

	return m, nil
}

func (m Model) updateTargetCreating(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.target.creating = false
			m.target.createErr = ""
			m.target.input.Blur()
			return m, nil

		case tea.KeyEnter:
			name := strings.TrimSpace(m.target.input.Value())
			if name == "" {
				m.target.createErr = "имя папки не может быть пустым"
				return m, nil
			}
			if strings.ContainsAny(name, "/\\\x00") {
				m.target.createErr = "имя содержит запрещённые символы"
				return m, nil
			}

			newPath := filepath.Join(m.target.currentDir, name)
			if err := os.MkdirAll(newPath, 0750); err != nil {
				m.target.createErr = err.Error()
				return m, nil
			}

			m.target.creating = false
			m.target.createErr = ""
			m.target.input.Blur()

			items, err := loadDirItems(m.target.currentDir)
			m.target.items = items
			if err != nil {
				m.target.readErr = err.Error()
			} else {
				m.target.readErr = ""
			}

			// Установить курсор на новую папку
			for i, item := range m.target.items {
				if item.name == name {
					m.target.cursor = i
					break
				}
			}
			return m, nil

		default:
		}
	}

	var cmd tea.Cmd
	m.target.input, cmd = m.target.input.Update(msg)
	return m, cmd
}

func (m Model) viewTarget() string {
	var b strings.Builder

	b.WriteString(m.theme.Title.Render(" photo-sorter "))
	b.WriteString("\n\n")
	b.WriteString(m.theme.Subtitle.Render("Шаг 2. Выбор целевой папки"))
	b.WriteString("\n\n")

	// ── Выбранные источники (read-only) ──
	b.WriteString(m.theme.Highlight.Render("Источники: "))
	if len(m.Sources) == 0 {
		b.WriteString("(не выбрано)\n")
	} else {
		b.WriteString(strings.Join(m.Sources, ", ") + "\n")
	}
	b.WriteString("\n")

	// ── Блок выбора цели ──
	b.WriteString(m.theme.Highlight.Render("Текущая папка: "))
	b.WriteString(m.target.currentDir)
	b.WriteString("\n\n")

	// Список папок
	if m.target.readErr != "" {
		b.WriteString(m.theme.Error.Render("  Ошибка чтения: "+m.target.readErr) + "\n")
	} else {
		for i, item := range m.target.items {
			cursor := "  "
			if m.target.cursor == i {
				cursor = m.theme.Highlight.Render("▸ ")
			}

			icon := "📁"
			if item.isParent {
				icon = "⬆️"
			}

			b.WriteString(fmt.Sprintf("%s%s %s\n", cursor, icon, item.name))
		}

		if len(m.target.items) == 0 {
			b.WriteString(m.theme.Error.Render("  (папка пуста)\n"))
		}
	}

	b.WriteString("\n")

	// ── Выбранная цель ──
	b.WriteString(m.theme.Highlight.Render("Цель: "))
	if m.Target == "" {
		b.WriteString("(не выбрано)\n")
	} else {
		b.WriteString(m.Target + "\n")
	}

	b.WriteString("\n")

	// Режим создания новой папки
	if m.target.creating {
		b.WriteString(m.theme.Highlight.Render("Имя новой папки:") + "\n")
		b.WriteString(m.target.input.View() + "\n")
		if m.target.createErr != "" {
			b.WriteString(m.theme.Error.Render("  Ошибка: "+m.target.createErr) + "\n")
		}
		b.WriteString("\n")
		b.WriteString(m.theme.Help.Render("enter — создать • esc — отмена"))
		return b.String()
	}

	nextHint := m.theme.Help.Render("→ — продолжить »")
	if m.Target == "" {
		nextHint = m.theme.Help.Render("→ — продолжить (выберите целевую папку)")
	}

	b.WriteString(m.theme.Help.Render(
		"↑/↓ — выбрать • enter — открыть • backspace — вверх • пробел/t — выбрать цель • c — выбрать текущую • n — новая папка • ← — назад • → — продолжить • esc — выход",
	))
	b.WriteString("\n")
	b.WriteString(nextHint)

	return b.String()
}
