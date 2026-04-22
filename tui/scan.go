package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"photo-sorter/internal/dateresolver"
	"photo-sorter/internal/deduper"
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
	exts := m.scanExtensions()
	layout := m.GetSettingString("template")

	return func() tea.Msg {
		// 1. Scan
		s := scanner.New([]string{m.Source}, exts...)
		files, err := s.Scan()
		if err != nil {
			return scanResultMsg{err: err}
		}

		// 2. Deduper
		d := deduper.New(files)
		duplicates := d.FindDuplicates()

		// 3. Sorter with date resolver
		dr := dateresolver.New()
		sort := sorter.New(m.Target, layout)
		entries := sort.BuildTree(files, duplicates, dr.Resolve)

		return scanResultMsg{
			files:      files,
			duplicates: duplicates,
			entries:    entries,
		}
	}
}

func (m Model) scanExtensions() []string {
	video := m.GetSettingBool("include_video")
	if video {
		return []string{".jpg", ".jpeg", ".png", ".heic", ".heif", ".mov", ".mp4", ".avi", ".mkv"}
	}
	return []string{".jpg", ".jpeg", ".png", ".heic", ".heif"}
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
		if m.scan.done || m.scan.errMsg != "" {
			switch msg.String() {
			case "enter":
				if m.scan.errMsg != "" {
					m.scan = newScanModel()
					m.scan.running = true
					return m, tea.Batch(scanTickCmd(), m.startScan())
				}
				m.screen = ScreenPreview
				return m, nil
			case "esc":
				return m, tea.Quit
			}
		} else {
			switch msg.Type {
			case tea.KeyCtrlC, tea.KeyEsc:
				return m, tea.Quit
			case tea.KeyLeft:
				if !m.scan.running {
					m.screen = ScreenSettings
					return m, nil
				}
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
		b.WriteString(helpStyle.Render("enter — предпросмотр • esc — выход"))
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
