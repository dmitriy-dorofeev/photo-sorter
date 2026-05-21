package tui

import (
	"context"
	"sync/atomic"

	"photo-sorter/internal/deduper"
	"photo-sorter/internal/depcheck"
	"photo-sorter/internal/scanner"
	"photo-sorter/internal/sorter"
	"photo-sorter/internal/state"
	"photo-sorter/internal/updater"

	"github.com/charmbracelet/bubbles/textinput"
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
	ScreenDeps
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
	deps       depsModel

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

	// Face preview (только при sort_mode == "face")
	faceAliasList        []string            // отсортированный список уникальных alias'ов
	faceAliasCursor      int                 // позиция курсора
	faceAliasRenaming    bool                // режим редактирования имени
	faceAliasInput       textinput.Model     // поле ввода нового имени
	faceAliasSamples     map[string][]string // примеры файлов (basename) для каждого alias'а
	faceAliasFullSamples map[string]string   // полный путь к первому файлу alias'а (для просмотра)
	faceAliasViewing     bool                // режим просмотра примера файла

	// Копирование
	copyCancel   context.CancelFunc
	copyProgress *atomic.Int64
	copyTotal    *atomic.Int64

	// Сканирование
	scanCancel     context.CancelFunc
	scanGeneration int

	// Face-кластеризация
	faceAliases map[string]string

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
		deps:         newDepsModel(),
		copyProgress: new(atomic.Int64),
		copyTotal:    new(atomic.Int64),
		theme:        NewLightTheme(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.sources.Init(),
		checkUpdateCmd(m.version),
		checkDepsCmd(),
		detectThemeCmd(),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case updateCheckMsg:
		m.updateResult = &msg.result
		return m, nil
	case depsCheckMsg:
		m.deps.results = msg.results
		for _, r := range msg.results {
			if r.Name == "exiftool" {
				if r.Status == depcheck.StatusOK {
					m.exifToolPath = r.Details
				} else {
					m.exifToolPath = ""
				}
			}
		}
		if len(msg.results.FilterMissing()) > 0 {
			m.screen = ScreenDeps
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
		m.deps.width = msg.Width
		m.deps.height = msg.Height
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
	case ScreenDeps:
		return m.updateDeps(msg)
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
	case ScreenDeps:
		return m.viewDeps()
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
	m.faceAliases = nil
	m.previewDirCache = nil
	m.previewCountCache = nil
	m.previewFileCache = nil
	m.faceAliasList = nil
	m.faceAliasCursor = 0
	m.faceAliasRenaming = false
	m.faceAliasInput = textinput.Model{}
	m.faceAliasSamples = nil
	m.faceAliasFullSamples = nil
	m.faceAliasViewing = false
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
