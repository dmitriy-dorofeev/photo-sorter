package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"photo-sorter/internal/deduper"
	"photo-sorter/internal/scanner"
	"photo-sorter/internal/sorter"
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
	Source string
	Target string

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

	// Копирование
	copyCancel context.CancelFunc
}

// NewModel создаёт новую модель, начиная с экрана выбора источников.
func NewModel() Model {
	return Model{
		screen:   ScreenSources,
		sources:  newSourcesModel(),
		target:   newTargetModel(),
		settings: newSettingsModel(),
		scan:     newScanModel(),
		copy:     newCopyModel(),
	}
}

func (m Model) Init() tea.Cmd {
	return m.sources.Init()
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
	}
	return m, nil
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
	}
	return ""
}

func (m Model) resetToSources() (tea.Model, tea.Cmd) {
	m.screen = ScreenSources
	m.Source = ""
	m.Target = ""
	m.files = nil
	m.duplicates = nil
	m.entries = nil
	m.sources = newSourcesModel()
	m.target = newTargetModel()
	m.settings = newSettingsModel()
	m.scan = newScanModel()
	m.copy = newCopyModel()
	return m, nil
}
