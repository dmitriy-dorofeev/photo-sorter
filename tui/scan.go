package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// scanTickMsg — сообщение тикера прогресса.
type scanTickMsg time.Time

func scanTickCmd() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return scanTickMsg(t)
	})
}

// scanModel — состояние экрана сканирования.
type scanModel struct {
	progress float64 // 0..100
	done     bool
	errorMsg string
}

func newScanModel() scanModel {
	return scanModel{}
}

func (s scanModel) Init() tea.Cmd {
	return scanTickCmd()
}

func (m Model) updateScan(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case scanTickMsg:
		if m.scan.done || m.scan.errorMsg != "" {
			return m, nil
		}
		m.scan.progress += 100.0 / 60.0 // 60 тиков × 50мс = 3с
		if m.scan.progress >= 100 {
			m.scan.progress = 100
			m.scan.done = true
			return m, nil
		}
		return m, scanTickCmd()

	case tea.KeyMsg:
		if m.scan.done || m.scan.errorMsg != "" {
			switch msg.String() {
			case "enter":
				return m.resetToSources()
			case "esc":
				return m, tea.Quit
			}
		} else {
			switch msg.Type {
			case tea.KeyCtrlC, tea.KeyEsc:
				return m, tea.Quit
			case tea.KeyLeft:
				m.screen = ScreenSettings
				return m, nil
			}
		}
	}

	return m, nil
}

func (m Model) viewScan() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(" photo-sorter "))
	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render("Шаг 4. Сканирование файлов"))
	b.WriteString("\n\n")

	// Предупреждение о незавершённом функционале
	b.WriteString(errorStyle.Render("⚠ Внимание: "))
	b.WriteString("сканирование пока в разработке. Функционал скоро будет доступен.\n\n")

	if m.scan.errorMsg != "" {
		b.WriteString(errorStyle.Render("Ошибка: "+m.scan.errorMsg) + "\n\n")
		b.WriteString(helpStyle.Render("enter — попробовать снова • esc — выход"))
		return b.String()
	}

	// Прогресс-бар
	barWidth := 30
	filled := int(m.scan.progress / 100.0 * float64(barWidth))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	b.WriteString(fmt.Sprintf("[%s] %.0f%%\n", bar, m.scan.progress))

	if m.scan.done {
		b.WriteString("\n")
		b.WriteString(successStyle.Render("✓ Процесс завершён!"))
		b.WriteString("\n\n")
		b.WriteString("Реальное сканирование и копирование будут реализованы в следующих версиях.\n\n")
		b.WriteString(helpStyle.Render("enter — начать заново • esc — выход"))
	} else {
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("← — назад • esc — отмена"))
	}

	return b.String()
}

func (m Model) resetToSources() (tea.Model, tea.Cmd) {
	m.screen = ScreenSources
	m.Source = ""
	m.Target = ""
	m.sources = newSourcesModel()
	m.target = newTargetModel()
	m.settings = newSettingsModel()
	m.scan = newScanModel()
	return m, nil
}
