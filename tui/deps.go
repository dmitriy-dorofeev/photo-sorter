package tui

import (
	"fmt"
	"runtime"
	"strings"

	"photo-sorter/internal/depcheck"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// depsCheckMsg передаёт результаты проверки зависимостей в модель.
type depsCheckMsg struct {
	results depcheck.Results
}

func checkDepsCmd() tea.Cmd {
	return func() tea.Msg {
		return depsCheckMsg{results: depcheck.CheckAll()}
	}
}

// depsModel — состояние экрана проверки зависимостей.
type depsModel struct {
	width   int
	height  int
	cursor  int // 0 = продолжить, 1 = выход
	results depcheck.Results
}

func newDepsModel() depsModel {
	return depsModel{
		cursor: 0,
	}
}

func (m Model) updateDeps(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyUp:
			if m.deps.cursor > 0 {
				m.deps.cursor--
			}
			return m, nil
		case tea.KeyDown:
			if m.deps.cursor < 1 {
				m.deps.cursor++
			}
			return m, nil
		case tea.KeyEnter:
			if m.deps.cursor == 0 {
				m.screen = ScreenSources
				return m, nil
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) viewDeps() string {
	var b strings.Builder

	b.WriteString(m.theme.Title.Render(" photo-sorter "))
	b.WriteString("\n\n")
	b.WriteString(m.theme.Subtitle.Render("Проверка зависимостей"))
	b.WriteString("\n\n")

	for _, r := range m.deps.results {
		var icon, statusStr string
		if r.Status == depcheck.StatusOK {
			icon = m.theme.Success.Render("✓")
			statusStr = m.theme.Success.Render("найден")
		} else {
			if r.Optional {
				icon = m.theme.Error.Render("⚠")
				statusStr = m.theme.Error.Render("не найден (опционально)")
			} else {
				icon = m.theme.Error.Render("✗")
				statusStr = m.theme.Error.Render("не найден")
			}
		}

		name := m.theme.Highlight.Render(r.Name)
		desc := r.Description
		details := m.theme.Help.Render(r.Details)

		line := lipgloss.JoinHorizontal(lipgloss.Top,
			fmt.Sprintf("%s %-18s", icon, name),
			statusStr,
			"  ",
			desc,
		)
		b.WriteString(line + "\n")
		if r.Status == depcheck.StatusMissing {
			b.WriteString("   " + details + "\n")
		}
	}

	missing := m.deps.results.FilterMissing()
	if len(missing) > 0 {
		b.WriteString("\n")
		b.WriteString(m.theme.Subtitle.Render("Установка:") + "\n")
		for _, r := range missing {
			if hint, ok := r.InstallHint[runtime.GOOS]; ok && hint != "" {
				b.WriteString(m.theme.Highlight.Render("  "+r.Name+":") + "\n")
				for _, line := range strings.Split(hint, "\n") {
					b.WriteString("    " + line + "\n")
				}
			}
		}
		b.WriteString("\n")
	}

	options := []string{"Продолжить", "Выход"}
	for i, opt := range options {
		cursor := "  "
		if m.deps.cursor == i {
			cursor = m.theme.Highlight.Render("▸ ")
		}
		b.WriteString(cursor + opt + "\n")
	}

	b.WriteString("\n")
	b.WriteString(m.theme.Help.Render("↑/↓ — выбрать • enter — подтвердить • esc — выход"))

	return b.String()
}
