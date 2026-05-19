package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// quickStartModel — состояние экрана быстрого старта.
type quickStartModel struct {
	width  int
	height int
	cursor int // 0 — начать сортировку, 1 — расширенные настройки
}

func newQuickStartModel() quickStartModel {
	return quickStartModel{
		cursor: 0,
	}
}

func (q quickStartModel) Init() tea.Cmd {
	return nil
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func (m Model) updateQuickStart(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyUp:
			if m.quickStart.cursor > 0 {
				m.quickStart.cursor--
			}
			return m, nil

		case tea.KeyDown:
			if m.quickStart.cursor < 1 {
				m.quickStart.cursor++
			}
			return m, nil

		case tea.KeyLeft:
			m.screen = ScreenTarget
			return m, nil

		case tea.KeyRight, tea.KeyEnter:
			if m.quickStart.cursor == 0 {
				// Быстрый старт — сразу запускаем сканирование
				m.screen = ScreenScan
				m.scan = newScanModel()
				m.scan.running = true
				m.scan.progressCh = make(chan runnerProgressMsg, 10)
				m, cmd := m.startScan()
				return m, tea.Batch(progressListenCmd(m.scan.progressCh), cmd)
			}
			// Расширенные настройки
			m.screen = ScreenSettings
			return m, nil
		default:
		}
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m Model) viewQuickStart() string {
	var b strings.Builder

	b.WriteString(m.theme.Title.Render(" photo-sorter "))
	b.WriteString("\n\n")
	b.WriteString(m.theme.Subtitle.Render("Шаг 3. Подтверждение"))
	b.WriteString("\n\n")

	// ── Сводка выбранных путей ──
	b.WriteString(m.theme.Highlight.Render("Источники: "))
	if len(m.Sources) == 0 {
		b.WriteString("(не выбрано)\n")
	} else {
		b.WriteString(strings.Join(m.Sources, ", ") + "\n")
	}
	b.WriteString(m.theme.Highlight.Render("Цель: "))
	b.WriteString(m.Target + "\n")
	b.WriteString("\n")

	// ── Краткая сводка настроек по умолчанию ──
	b.WriteString(m.theme.Subtitle.Render("Настройки по умолчанию:"))
	b.WriteString("\n")

	templateVal := m.GetSettingString("template")
	b.WriteString(fmt.Sprintf("  • Шаблон папок: %s\n", formatTemplateDisplay(templateVal)))

	collisionVal := m.GetSettingString("collision_strategy")
	collisionLabel := collisionVal
	for _, s := range m.settings.items {
		if s.key == "collision_strategy" && s.stype == settingTypeChoice {
			collisionLabel = s.choices[s.choiceIdx]
			break
		}
	}
	b.WriteString(fmt.Sprintf("  • Конфликты имён: %s\n", collisionLabel))

	if m.GetSettingBool("skip_sorted") {
		b.WriteString("  • Пропускать уже отсортированные: да\n")
	} else {
		b.WriteString("  • Пропускать уже отсортированные: нет\n")
	}

	b.WriteString("\n")

	// ── Опции ──
	options := []struct {
		label string
		icon  string
	}{
		{"Начать сортировку", "⚡"},
		{"Расширенные настройки", "⚙️"},
	}

	for i, opt := range options {
		cursor := "  "
		if m.quickStart.cursor == i {
			cursor = m.theme.Highlight.Render("▸ ")
		}
		b.WriteString(fmt.Sprintf("%s%s %s\n", cursor, opt.icon, opt.label))
	}

	b.WriteString("\n")
	b.WriteString(m.theme.Help.Render("↑/↓ — выбрать • enter/→ — продолжить • ← — назад • esc — выход"))

	return b.String()
}
