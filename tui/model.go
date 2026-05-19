package tui

import (
	"context"
	"sync/atomic"

	"photo-sorter/internal/dateresolver"
	"photo-sorter/internal/deduper"
	"photo-sorter/internal/scanner"
	"photo-sorter/internal/sorter"
	"photo-sorter/internal/state"
	"photo-sorter/internal/updater"

	tea "github.com/charmbracelet/bubbletea"
)

// Screen описывает текущий экран приложения.
type Screen int

const (
	ScreenSources Screen = iota
	ScreenTarget
	ScreenQuickStart
	ScreenSettings
	ScreenScan
	ScreenPreview
	ScreenCopy
)

// Model — главная модель bubbletea.
type Model struct {
	screen Screen
	width  int
	height int

	// Данные, общие для всех экранов
	Sources []string
	Target  string

	// Экраны (внутренние модели)
	sources    sourcesModel
	target     targetModel
	quickStart quickStartModel
	settings   settingsModel
	scan       scanModel
	copy       copyModel

	// Результаты сканирования
	files      []scanner.FileInfo
	duplicates []deduper.Result
	entries    []sorter.Entry

	// Данные для инкрементальности
	fastHashes map[string]uint64
	fullHashes map[string]uint64
	allPaths   []string
	st         *state.State

	// Кэш для preview.go (пересчитывается при изменении entries)
	previewDirCache   []string
	previewCountCache map[string]int
	previewFileCache  map[string][]string

	// Копирование
	copyCancel   context.CancelFunc
	copyProgress *atomic.Int64
	copyTotal    *atomic.Int64

	// Сканирование
	scanCancel     context.CancelFunc
	scanGeneration int

	// Версия и обновление
	version      string
	updateResult *updater.CheckResult

	// Доступность exiftool
	exifToolPath string

	// Тема оформления
	theme *Theme
}

// NewModel создаёт новую модель, начиная с экрана выбора источников.
func NewModel(version string) Model {
	return Model{
		screen:       ScreenSources,
		version:      version,
		sources:      newSourcesModel(),
		target:       newTargetModel(),
		quickStart:   newQuickStartModel(),
		settings:     newSettingsModel(),
		scan:         newScanModel(),
		copy:         newCopyModel(),
		copyProgress: new(atomic.Int64),
		copyTotal:    new(atomic.Int64),
		theme:        NewLightTheme(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.sources.Init(),
		checkUpdateCmd(m.version),
		checkExifToolCmd(),
		detectThemeCmd(),
	)
}

// exifToolCheckMsg передаёт результат проверки наличия exiftool.
type exifToolCheckMsg struct {
	path string
	ok   bool
}

func checkExifToolCmd() tea.Cmd {
	return func() tea.Msg {
		path, ok := dateresolver.FindExifTool()
		return exifToolCheckMsg{path: path, ok: ok}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case updateCheckMsg:
		m.updateResult = &msg.result
		return m, nil
	case exifToolCheckMsg:
		m.exifToolPath = msg.path
		if !msg.ok {
			m.exifToolPath = ""
		}
		return m, nil
	case themeMsg:
		m.applyThemeMode(msg.mode)
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.sources.width = msg.Width
		m.sources.height = msg.Height
		m.target.width = msg.Width
		m.target.height = msg.Height
		m.quickStart.width = msg.Width
		m.quickStart.height = msg.Height
		m.settings.width = msg.Width
		m.settings.height = msg.Height
		m.scan.width = msg.Width
		m.scan.height = msg.Height
		m.copy.width = msg.Width
		m.copy.height = msg.Height
		return m, nil
	}

	switch m.screen {
	case ScreenSources:
		return m.updateSources(msg)
	case ScreenTarget:
		return m.updateTarget(msg)
	case ScreenQuickStart:
		return m.updateQuickStart(msg)
	case ScreenSettings:
		return m.updateSettings(msg)
	case ScreenScan:
		return m.updateScan(msg)
	case ScreenPreview:
		return m.updatePreview(msg)
	case ScreenCopy:
		return m.updateCopy(msg)
	default:
		m.screen = ScreenSources
		return m, tea.Quit
	}
}

func (m Model) View() string {
	switch m.screen {
	case ScreenSources:
		return m.viewSources()
	case ScreenTarget:
		return m.viewTarget()
	case ScreenQuickStart:
		return m.viewQuickStart()
	case ScreenSettings:
		return m.viewSettings()
	case ScreenScan:
		return m.viewScan()
	case ScreenPreview:
		return m.viewPreview()
	case ScreenCopy:
		return m.viewCopy()
	default:
		return m.theme.Error.Render("Неизвестный экран. Нажмите любую клавишу для выхода.")
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
	m.previewFileCache = nil
	m.sources = newSourcesModel()
	m.target = newTargetModel()
	m.quickStart = newQuickStartModel()
	m.settings = newSettingsModel()
	m.scan = newScanModel()
	m.copy = newCopyModel()
	m.copyProgress = new(atomic.Int64)
	m.copyTotal = new(atomic.Int64)
	return m, nil
}

// applyThemeMode применяет тему в зависимости от режима.
func (m *Model) applyThemeMode(mode ThemeMode) {
	switch mode {
	case ThemeModeDark:
		m.theme = NewDarkTheme()
	case ThemeModeLight:
		m.theme = NewLightTheme()
	default:
		if systemThemeMode() == ThemeModeDark {
			m.theme = NewDarkTheme()
		} else {
			m.theme = NewLightTheme()
		}
	}
}
