package tui

import (
	tea "github.com/charmbracelet/bubbletea"
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
}

// NewModel создаёт новую модель, начиная с экрана выбора источников.
func NewModel() Model {
	return Model{
		screen:   ScreenSources,
		sources:  newSourcesModel(),
		target:   newTargetModel(),
		settings: newSettingsModel(),
		scan:     newScanModel(),
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
		// TODO: экран предпросмотра
		return m, nil
	case ScreenCopy:
		// TODO: экран выполнения
		return m, nil
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
