package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"photo-sorter/internal/deduper"
	"photo-sorter/internal/runner"
	"photo-sorter/internal/scanner"
	"photo-sorter/internal/sorter"
)

// ---------------------------------------------------------------------------
// Сообщения
// ---------------------------------------------------------------------------

type scanTickMsg time.Time

type scanResultMsg struct {
	files      []scanner.FileInfo
	duplicates []deduper.Result
	entries    []sorter.Entry
	err        error
}

func scanTickCmd() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return scanTickMsg(t)
	})
}

// ---------------------------------------------------------------------------
// Модель
// ---------------------------------------------------------------------------

type scanStage int

const (
	scanStageScanning scanStage = iota
	scanStageDates
	scanStageDedup
	scanStageTree
	scanStageDone
)

var scanStageNames = []string{
	"Сканирование файлов...",
	"Определение дат съёмки...",
	"Поиск дубликатов...",
	"Построение дерева папок...",
}

type scanModel struct {
	running  bool
	stage    scanStage
	progress float64 // 0..100
	done     bool
	errMsg   string
	aborted  bool // true если пользователь прервал или ушёл назад
}

func newScanModel() scanModel {
	return scanModel{}
}

func (s scanModel) Init() tea.Cmd {
	return nil
}

// ---------------------------------------------------------------------------
// Запуск сканирования
// ---------------------------------------------------------------------------

func (m Model) startScan() tea.Cmd {
	if m.scanCancel != nil {
		m.scanCancel()
	}

	cfg := runner.Config{
		Sources:      []string{m.Source},
		Target:       m.Target,
		Template:     m.GetSettingString("template"),
		LivePhotos:   m.GetSettingBool("live_photos"),
		IncludeVideo: m.GetSettingBool("include_video"),
		UseMTime:     m.GetSettingBool("use_mtime"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.scanCancel = cancel

	return func() tea.Msg {
		res, err := runner.Run(ctx, cfg, nil)
		if err != nil {
			return scanResultMsg{err: err}
		}

		return scanResultMsg{
			files:      res.Files,
			duplicates: res.Duplicates,
			entries:    res.Entries,
		}
	}
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func (m Model) updateScan(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case scanTickMsg:
		if m.scan.done || m.scan.errMsg != "" || !m.scan.running {
			return m, nil
		}
		m.scan.progress += 3
		if m.scan.progress >= 95 {
			m.scan.progress = 95
		}
		stageIdx := int(m.scan.progress / 25)
		if stageIdx > 3 {
			stageIdx = 3
		}
		m.scan.stage = scanStage(stageIdx)
		return m, scanTickCmd()

	case scanResultMsg:
		if m.scan.aborted {
			return m, nil
		}
		m.scan.running = false
		if msg.err != nil {
			m.scan.errMsg = msg.err.Error()
			return m, nil
		}
		m.scan.done = true
		m.scan.progress = 100
		m.scan.stage = scanStageDone
		m.files = msg.files
		m.duplicates = msg.duplicates
		m.entries = msg.entries
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			if m.scanCancel != nil {
				m.scanCancel()
			}
			m.scan.aborted = true
			return m, tea.Quit
		case tea.KeyLeft:
			if m.scanCancel != nil {
				m.scanCancel()
			}
			m.scan.aborted = true
			m.scan = newScanModel()
			m.screen = ScreenSettings
			return m, nil
		}
		switch msg.String() {
		case "enter":
			if m.scan.errMsg != "" {
				if m.scanCancel != nil {
					m.scanCancel()
				}
				m.scan = newScanModel()
				m.scan.running = true
				return m, tea.Batch(scanTickCmd(), m.startScan())
			}
			if m.scan.done {
				m.screen = ScreenPreview
				return m, nil
			}
		}
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m Model) viewScan() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(" photo-sorter "))
	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render("Шаг 4. Сканирование файлов"))
	b.WriteString("\n\n")

	if m.scan.errMsg != "" {
		b.WriteString(errorStyle.Render("Ошибка: "+m.scan.errMsg) + "\n\n")
		b.WriteString(helpStyle.Render("enter — попробовать снова • esc — выход"))
		return b.String()
	}

	barWidth := 30
	filled := int(m.scan.progress / 100.0 * float64(barWidth))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	b.WriteString(fmt.Sprintf("[%s] %.0f%%\n", bar, m.scan.progress))
	b.WriteString("\n")

	if m.scan.stage >= 0 && int(m.scan.stage) < len(scanStageNames) {
		b.WriteString(scanStageNames[m.scan.stage] + "\n")
	}

	if m.scan.done {
		st := m.computeScanStats()
		b.WriteString("\n")
		b.WriteString(successStyle.Render("✓ Сканирование завершено!"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("Найдено файлов: %d\n", st.total))
		b.WriteString(fmt.Sprintf("Определено дат: %d\n", st.withDate))
		b.WriteString(fmt.Sprintf("Без даты (unsorted): %d\n", st.unsorted))
		b.WriteString(fmt.Sprintf("Дубликатов: %d\n", st.duplicates))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("enter — предпросмотр • ← — назад • esc — выход"))
	} else if m.scan.running {
		b.WriteString(helpStyle.Render("← — назад • esc — отмена"))
	}

	return b.String()
}

type scanStats struct {
	total      int
	withDate   int
	unsorted   int
	duplicates int
}

func (m Model) computeScanStats() scanStats {
	var s scanStats
	s.total = len(m.files)
	for _, e := range m.entries {
		if e.Skip {
			s.duplicates++
		} else if strings.Contains(e.Target, "unsorted") {
			s.unsorted++
		} else {
			s.withDate++
		}
	}
	return s
}
