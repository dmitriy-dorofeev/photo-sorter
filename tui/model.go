package tui

import (
	"context"
	"fmt"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
	"photo-sorter/internal/deduper"
	"photo-sorter/internal/scanner"
	"photo-sorter/internal/sorter"
	"photo-sorter/internal/updater"
)

// Screen описывает текущий экран приложения.
type Screen int

const (
	ScreenSources Screen = iota
	ScreenTarget
	ScreenSettings
	ScreenScan
	ScreenPreview
	ScreenCopy
)

// Model — главная модель bubbletea.
type Model struct {
	screen Screen

	// Данные, общие для всех экранов
	Sources []string
	Target  string

	// Экраны (внутренние модели)
	sources  sourcesModel
	target   targetModel
	settings settingsModel
	scan     scanModel
	copy     copyModel

	// Результаты сканирования
	files      []scanner.FileInfo
	duplicates []deduper.Result
	entries    []sorter.Entry

	// Кэш для preview.go (пересчитывается при изменении entries)
	previewDirCache   []string
	previewCountCache map[string]int

	// Копирование
	copyCancel   context.CancelFunc
	copyProgress *atomic.Int64
	copyTotal    *atomic.Int64

	// Сканирование
	scanCancel context.CancelFunc

	// Версия и обновление
	version      string
	updateResult *updater.CheckResult
}

// NewModel создаёт новую модель, начиная с экрана выбора источников.
func NewModel(version string) Model {
	return Model{
		screen:       ScreenSources,
		version:      version,
		sources:      newSourcesModel(),
		target:       newTargetModel(),
		settings:     newSettingsModel(),
		scan:         newScanModel(),
		copy:         newCopyModel(),
		copyProgress: new(atomic.Int64),
		copyTotal:    new(atomic.Int64),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.sources.Init(),
		checkUpdateCmd(m.version),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case ScreenSources:
		return m.updateSources(msg)
	case ScreenTarget:
		return m.updateTarget(msg)
	case ScreenSettings:
		return m.updateSettings(msg)
	case ScreenScan:
		return m.updateScan(msg)
	case ScreenPreview:
		return m.updatePreview(msg)
	case ScreenCopy:
		return m.updateCopy(msg)
	default:
		panic(fmt.Sprintf("unknown screen: %d", m.screen))
	}
}

func (m Model) View() string {
	switch m.screen {
	case ScreenSources:
		return m.viewSources()
	case ScreenTarget:
		return m.viewTarget()
	case ScreenSettings:
		return m.viewSettings()
	case ScreenScan:
		return m.viewScan()
	case ScreenPreview:
		return m.viewPreview()
	case ScreenCopy:
		return m.viewCopy()
	default:
		panic(fmt.Sprintf("unknown screen: %d", m.screen))
	}
}

func (m Model) resetToSources() (tea.Model, tea.Cmd) {
	if m.copyCancel != nil {
		m.copyCancel()
	}
	if m.scanCancel != nil {
		m.scanCancel()
	}
	m.screen = ScreenSources
	m.Sources = nil
	m.Target = ""
	m.files = nil
	m.duplicates = nil
	m.entries = nil
	m.previewDirCache = nil
	m.previewCountCache = nil
	m.sources = newSourcesModel()
	m.target = newTargetModel()
	m.settings = newSettingsModel()
	m.scan = newScanModel()
	m.copy = newCopyModel()
	m.copyProgress = new(atomic.Int64)
	m.copyTotal = new(atomic.Int64)
	return m, nil
}
