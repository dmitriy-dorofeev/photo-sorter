package tui

import (
	"context"
	"fmt"
	"strings"

	"photo-sorter/internal/deduper"
	"photo-sorter/internal/runner"
	"photo-sorter/internal/scanner"
	"photo-sorter/internal/sorter"

	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// Сообщения
// ---------------------------------------------------------------------------

type scanResultMsg struct {
	files      []scanner.FileInfo
	duplicates []deduper.Result
	entries    []sorter.Entry
	err        error
	generation int
}

// runnerProgressMsg передаёт прогресс из runner.Run в TUI.
type runnerProgressMsg struct {
	stage   string
	current int
	total   int
}

// progressListenCmd читает прогресс из канала и возвращает его как сообщение.
// Когда канал закрыт или nil — возвращает nil.
func progressListenCmd(ch <-chan runnerProgressMsg) tea.Cmd {
	return func() tea.Msg {
		if ch == nil {
			return nil
		}
		p, ok := <-ch
		if !ok {
			return nil
		}
		return p
	}
}

// ---------------------------------------------------------------------------
// Модель
// ---------------------------------------------------------------------------

type scanStage int

const (
	scanStageScanning scanStage = iota
	scanStageDedup
	scanStageTree
	scanStageDone
)

var scanStageNames = []string{
	"Сканирование файлов...",
	"Поиск дубликатов...",
	"Построение дерева папок...",
	"Готово",
}

type scanModel struct {
	width      int
	height     int
	running    bool
	stage      scanStage
	progress   float64 // 0..100
	done       bool
	errMsg     string
	aborted    bool                   // true если пользователь прервал или ушёл назад
	progressCh chan runnerProgressMsg // канал для прогресса из runner.Run
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

func (m *Model) startScan() tea.Cmd {
	if m.scanCancel != nil {
		m.scanCancel()
	}

	cfg := runner.Config{
		Sources:      m.Sources,
		Target:       m.Target,
		Template:     m.GetSettingString("template"),
		LivePhotos:   m.GetSettingBool("live_photos"),
		IncludeVideo: m.GetSettingBool("include_video"),
		UseMTime:     m.GetSettingBool("use_mtime"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.scanCancel = cancel
	m.scanGeneration++
	gen := m.scanGeneration

	progressCh := m.scan.progressCh

	return func() tea.Msg {
		res, err := runner.Run(ctx, cfg, func(stage string, current, total int) {
			if progressCh == nil {
				return
			}
			select {
			case progressCh <- runnerProgressMsg{stage: stage, current: current, total: total}:
			case <-ctx.Done():
			}
		})
		if progressCh != nil {
			close(progressCh)
		}
		if err != nil {
			return scanResultMsg{err: err, generation: gen}
		}

		return scanResultMsg{
			files:      res.Files,
			duplicates: res.Duplicates,
			entries:    res.Entries,
			generation: gen,
		}
	}
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func (m Model) updateScan(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case runnerProgressMsg:
		if m.scan.aborted || m.scan.done || !m.scan.running {
			return m, nil
		}
		var pct float64
		switch msg.stage {
		case "scan":
			m.scan.stage = scanStageScanning
			pct = 0.30
		case "dedup":
			m.scan.stage = scanStageDedup
			pct = 0.30
		case "sort":
			m.scan.stage = scanStageTree
			pct = 0.30
		default:
			pct = 0.10
		}
		if msg.total > 0 {
			m.scan.progress = float64(msg.current) / float64(msg.total) * pct * 100
		} else {
			m.scan.progress = 0
		}
		// Добавляем базовый offset для завершённых этапов
		switch msg.stage {
		case "dedup":
			m.scan.progress += 30
		case "sort":
			m.scan.progress += 60
		}
		return m, progressListenCmd(m.scan.progressCh)

	case scanResultMsg:
		if m.scan.aborted || msg.generation != m.scanGeneration {
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
		m = buildPreviewCache(m)
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
				m.scan.progressCh = make(chan runnerProgressMsg, 10)
				return m, tea.Batch(progressListenCmd(m.scan.progressCh), m.startScan())
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
	st := runner.Result{Files: m.files, Entries: m.entries}.Stats()
	return scanStats{
		total:      st.Total,
		withDate:   st.WithDate,
		unsorted:   st.Unsorted,
		duplicates: st.Duplicates,
	}
}
