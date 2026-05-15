package tui

import (
	"fmt"
	"strings"
	"time"

	"photo-sorter/internal/config"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Пресеты шаблонов папок (понятные пользователю метки → Go time layouts)
// ---------------------------------------------------------------------------

type templatePreset struct {
	label    string // например "YYYY/MM/DD"
	value    string // Go layout, например "2006/01/02"
	isCustom bool   // true для "Свой формат…"
}

var templatePresets = []templatePreset{
	{label: "YYYY-MM-DD", value: "2006-01-02"},
	{label: "YYYY/MM/DD", value: "2006/01/02"},
	{label: "YYYY/MM", value: "2006/01"},
	{label: "YYYY", value: "2006"},
	{label: "Свой формат…", value: "", isCustom: true},
}

// findPreset ищет пресет по Go-значению. Возвращает индекс и found.
func findPreset(value string) (int, bool) {
	customIdx := -1
	for i, p := range templatePresets {
		if p.value == value {
			return i, true
		}
		if p.isCustom {
			customIdx = i
		}
	}
	if customIdx >= 0 {
		return customIdx, false
	}
	return 0, false
}

// formatTemplateDisplay возвращает человекочитаемое представление шаблона папок.
func formatTemplateDisplay(value string) string {
	if value == "" {
		return "—"
	}
	idx, found := findPreset(value)
	now := time.Now()
	example := now.Format(value)
	if found {
		return fmt.Sprintf("%s (%s)", templatePresets[idx].label, example)
	}
	return fmt.Sprintf("Свой формат (%s)", example)
}

// ---------------------------------------------------------------------------
// Пресеты шаблонов имён файлов
// ---------------------------------------------------------------------------

var fileNamePresets = []templatePreset{
	{label: "Оригинальное имя", value: "{original}{ext}"},
	{label: "Дата + оригинал", value: "{YYYY}-{MM}-{DD}_{original}{ext}"},
	{label: "Дата-время + оригинал", value: "{YYYY}-{MM}-{DD}_{HH}-{mm}-{SS}_{original}{ext}"},
	{label: "Компактная дата-время", value: "{YYYY}{MM}{DD}_{HH}{mm}{SS}{ext}"},
	{label: "Дата-время + устройство", value: "{YYYY}-{MM}-{DD}_{HH}-{mm}-{SS}_{device}{ext}"},
	{label: "Хронологический счётчик", value: "{YYYY}{MM}{DD}_{seq:03}{ext}"},
	{label: "Свой формат…", value: "", isCustom: true},
}

func findFileNamePreset(value string) (int, bool) {
	customIdx := -1
	for i, p := range fileNamePresets {
		if p.value == value {
			return i, true
		}
		if p.isCustom {
			customIdx = i
		}
	}
	if customIdx >= 0 {
		return customIdx, false
	}
	return 0, false
}

// formatFileNameDisplay возвращает человекочитаемое представление шаблона имён файлов.
func formatFileNameDisplay(value string) string {
	if value == "" {
		return "—"
	}
	idx, found := findFileNamePreset(value)
	if found {
		return fileNamePresets[idx].label
	}
	return fmt.Sprintf("Свой формат (%s)", value)
}

// ---------------------------------------------------------------------------
// Настройки
// ---------------------------------------------------------------------------

type settingType int

const (
	settingTypeText settingType = iota
	settingTypeBool
	settingTypeChoice
)

type setting struct {
	label        string
	key          string
	help         string
	stringValue  string
	boolValue    bool
	stype        settingType
	choices      []string
	choiceValues []string
	choiceIdx    int
}

// AsString возвращает строковое значение настройки.
// Если тип не settingTypeText и не settingTypeChoice, возвращает пустую строку.
func (s setting) AsString() string {
	if s.stype != settingTypeText && s.stype != settingTypeChoice {
		return ""
	}
	return s.stringValue
}

// AsBool возвращает boolean-значение настройки.
// Если тип не settingTypeBool, возвращает false.
func (s setting) AsBool() bool {
	if s.stype != settingTypeBool {
		return false
	}
	return s.boolValue
}

// settingsModel — состояние экрана настроек.
type settingsModel struct {
	width          int
	height         int
	cursor         int
	items          []setting
	editing        bool // true → редактируем custom через textinput
	templateSelect bool // true → показываем список пресетов папок
	templateCursor int  // курсор внутри списка пресетов папок
	fileNameSelect bool // true → показываем список пресетов имён файлов
	fileNameCursor int  // курсор внутри списка пресетов имён файлов
	input          textinput.Model
}

func newSettingsModel() settingsModel {
	ti := textinput.New()
	ti.Placeholder = "2006-01-02"
	ti.Width = 30

	return settingsModel{
		cursor: 0,
		items: []setting{
			{
				label:       "Шаблон папок",
				key:         "template",
				help:        "Формат именования папок по дате",
				stringValue: config.DefaultTemplate,
				stype:       settingTypeText,
			},
			{
				label:       "Шаблон имён файлов",
				key:         "file_name_template",
				help:        "Формат новых имён файлов",
				stringValue: config.DefaultFileNameTemplate,
				stype:       settingTypeText,
			},
			{
				label:     "Группировать Live Photos",
				key:       "live_photos",
				help:      "Не считать .heic + .mov дубликатами (Live Photos)",
				boolValue: config.DefaultLivePhotos,
				stype:     settingTypeBool,
			},
			{
				label:     "Включать видео",
				key:       "include_video",
				help:      "Обрабатывать видеофайлы",
				boolValue: config.DefaultIncludeVideo,
				stype:     settingTypeBool,
			},
			{
				label:     "Использовать дату изменения",
				key:       "use_mtime",
				help:      "Если нет EXIF/имени, использовать ModTime файла",
				boolValue: config.DefaultUseMTime,
				stype:     settingTypeBool,
			},
			{
				label:     "Записывать дату в EXIF",
				key:       "write_exif",
				help:      "Если дата взята из имени/mtime, записать её в метаданные файла",
				boolValue: config.DefaultWriteExif,
				stype:     settingTypeBool,
			},
			{
				label:     "Показывать уведомления",
				key:       "notify",
				help:      "Показать системное уведомление по завершении сортировки",
				boolValue: config.DefaultNotify,
				stype:     settingTypeBool,
			},
			{
				label:     "Пропускать уже отсортированные",
				key:       "skip_sorted",
				help:      "Не обрабатывать файлы, которые уже были отсортированы ранее",
				boolValue: true,
				stype:     settingTypeBool,
			},
			{
				label:        "Стратегия дедупликации",
				key:          "dup_strategy",
				help:         "Какой файл считать оригиналом при обнаружении дубликатов",
				stype:        settingTypeChoice,
				choices:      []string{"По имени файла", "По размеру", "По дате изменения", "По метаданным"},
				choiceValues: []string{"path", "largest", "newest", "best-meta"},
				choiceIdx:    0,
				stringValue:  "path",
			},
			{
				label:        "Конфликты имён",
				key:          "collision_strategy",
				help:         "Как формировать суффикс, если имя уже занято",
				stype:        settingTypeChoice,
				choices:      []string{"Счётчик (_1, _2)", "Короткий хеш"},
				choiceValues: []string{"counter", "hash"},
				choiceIdx:    0,
				stringValue:  "counter",
			},
		},
		input: ti,
	}
}

func (s settingsModel) Init() tea.Cmd {
	return nil
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func (m Model) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch {
	case m.settings.templateSelect:
		return m.updateTemplateSelect(msg)
	case m.settings.fileNameSelect:
		return m.updateFileNameTemplateSelect(msg)
	case m.settings.editing:
		return m.updateSettingsEditing(msg)
	default:
		return m.updateSettingsNav(msg)
	}
}

// Режим выбора пресета шаблона папок.
func (m Model) updateTemplateSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.settings.templateSelect = false
			return m, nil

		case tea.KeyUp:
			if m.settings.templateCursor > 0 {
				m.settings.templateCursor--
			}
			return m, nil

		case tea.KeyDown:
			if m.settings.templateCursor < len(templatePresets)-1 {
				m.settings.templateCursor++
			}
			return m, nil

		case tea.KeyEnter, tea.KeySpace:
			preset := templatePresets[m.settings.templateCursor]
			if preset.isCustom {
				m.settings.templateSelect = false
				m.settings.editing = true
				m.settings.input.SetValue(m.settings.items[m.settings.cursor].AsString())
				m.settings.input.Focus()
				return m, textinput.Blink
			}
			m.settings.items[m.settings.cursor].stringValue = preset.value
			m.settings.templateSelect = false
			return m, nil
		default:
			return m, nil
		}
	}
	return m, nil
}

// Режим выбора пресета шаблона имён файлов.
func (m Model) updateFileNameTemplateSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.settings.fileNameSelect = false
			return m, nil

		case tea.KeyUp:
			if m.settings.fileNameCursor > 0 {
				m.settings.fileNameCursor--
			}
			return m, nil

		case tea.KeyDown:
			if m.settings.fileNameCursor < len(fileNamePresets)-1 {
				m.settings.fileNameCursor++
			}
			return m, nil

		case tea.KeyEnter, tea.KeySpace:
			preset := fileNamePresets[m.settings.fileNameCursor]
			if preset.isCustom {
				m.settings.fileNameSelect = false
				m.settings.editing = true
				m.settings.input.SetValue(m.settings.items[m.settings.cursor].AsString())
				m.settings.input.Focus()
				return m, textinput.Blink
			}
			m.settings.items[m.settings.cursor].stringValue = preset.value
			m.settings.fileNameSelect = false
			return m, nil
		default:
			return m, nil
		}
	}
	return m, nil
}

// Режим ручного ввода (custom) через textinput.
func (m Model) updateSettingsEditing(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.settings.editing = false
			m.settings.input.Blur()
			return m, nil

		case tea.KeyEnter:
			m.settings.editing = false
			m.settings.input.Blur()
			m.settings.items[m.settings.cursor].stringValue = m.settings.input.Value()
			return m, nil
		default:
		}
	}

	var cmd tea.Cmd
	m.settings.input, cmd = m.settings.input.Update(msg)
	return m, cmd
}

// Обычная навигация по списку настроек.
func (m Model) updateSettingsNav(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyUp:
			if m.settings.cursor > 0 {
				m.settings.cursor--
			}
			return m, nil

		case tea.KeyDown:
			if m.settings.cursor < len(m.settings.items)-1 {
				m.settings.cursor++
			}
			return m, nil

		case tea.KeyLeft:
			m.screen = ScreenTarget
			return m, nil

		case tea.KeyRight:
			m.screen = ScreenScan
			m.scan = newScanModel()
			m.scan.running = true
			m.scan.progressCh = make(chan runnerProgressMsg, 10)
			m, cmd := m.startScan()
			return m, tea.Batch(progressListenCmd(m.scan.progressCh), cmd)
		default:
		}

		switch msg.String() {
		case " ", "enter":
			item := &m.settings.items[m.settings.cursor]
			switch item.stype {
			case settingTypeBool:
				item.boolValue = !item.boolValue
			case settingTypeText:
				if item.key == "template" {
					m.settings.templateSelect = true
					idx, _ := findPreset(item.stringValue)
					m.settings.templateCursor = idx
				} else if item.key == "file_name_template" {
					m.settings.fileNameSelect = true
					idx, _ := findFileNamePreset(item.stringValue)
					m.settings.fileNameCursor = idx
				}
				return m, nil
			case settingTypeChoice:
				item.choiceIdx++
				if item.choiceIdx >= len(item.choices) {
					item.choiceIdx = 0
				}
				item.stringValue = item.choiceValues[item.choiceIdx]
				return m, nil
			default:
				return m, nil
			}
			return m, nil

		}
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m Model) viewSettings() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(" photo-sorter "))
	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render("Шаг 3. Настройки сортировки"))
	b.WriteString("\n\n")

	// Показываем выбранные пути
	b.WriteString(highlightStyle.Render("Источники: "))
	if len(m.Sources) == 0 {
		b.WriteString("(не выбрано)\n")
	} else {
		b.WriteString(strings.Join(m.Sources, ", ") + "\n")
	}
	b.WriteString(highlightStyle.Render("Цель: "))
	b.WriteString(m.Target + "\n")
	b.WriteString("\n")

	// Если выбираем пресет — рисуем модальный список поверх
	if m.settings.templateSelect {
		b.WriteString(m.viewTemplateSelect())
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("↑/↓ — выбрать • enter — применить • esc — отмена"))
		return b.String()
	}
	if m.settings.fileNameSelect {
		b.WriteString(m.viewFileNameTemplateSelect())
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("↑/↓ — выбрать • enter — применить • esc — отмена"))
		return b.String()
	}

	// Список настроек
	for i, item := range m.settings.items {
		cursor := "  "
		if m.settings.cursor == i {
			cursor = highlightStyle.Render("▸ ")
		}

		var valueStr string
		switch item.stype {
		case settingTypeBool:
			if item.boolValue {
				valueStr = successStyle.Render("✓ да")
			} else {
				valueStr = errorStyle.Render("✗ нет")
			}
		case settingTypeText:
			if m.settings.editing && m.settings.cursor == i {
				valueStr = m.settings.input.View()
			} else {
				if item.key == "template" {
					valueStr = highlightStyle.Render(formatTemplateDisplay(item.stringValue))
				} else {
					valueStr = highlightStyle.Render(formatFileNameDisplay(item.stringValue))
				}
			}
		case settingTypeChoice:
			valueStr = highlightStyle.Render(item.choices[item.choiceIdx])
		default:
		}

		labelCol := settingLabelStyle.Render(cursor + item.label + ":")
		line := lipgloss.JoinHorizontal(lipgloss.Top, labelCol, valueStr)
		b.WriteString(line + "\n")

		if m.settings.cursor == i && !m.settings.editing {
			helpLine := helpStyle.Render("  " + item.help)
			b.WriteString(helpLine + "\n")
		}
	}

	b.WriteString("\n")

	if m.exifToolPath == "" {
		b.WriteString(errorStyle.Render("⚠ exiftool не найден: видео-метаданные и запись EXIF недоступны.") + "\n")
		b.WriteString("\n")
	}

	if m.settings.editing {
		b.WriteString(helpStyle.Render("enter — сохранить • esc — отменить"))
	} else {
		b.WriteString(helpStyle.Render("↑/↓ — выбрать • enter/пробел — изменить • ← — назад • → — продолжить • esc — выход"))
	}

	return b.String()
}

// viewTemplateSelect рисует список пресетов шаблонов папок.
func (m Model) viewTemplateSelect() string {
	var b strings.Builder

	b.WriteString(highlightStyle.Render("Выберите формат папок:"))
	b.WriteString("\n\n")

	for i, preset := range templatePresets {
		cursor := "  "
		if m.settings.templateCursor == i {
			cursor = highlightStyle.Render("▸ ")
		}

		if preset.label == "Свой формат…" {
			b.WriteString(cursor + preset.label + "\n")
		} else {
			example := time.Now().Format(preset.value)
			labelCol := templateLabelStyle.Render(cursor + preset.label)
			line := lipgloss.JoinHorizontal(lipgloss.Top, labelCol, " → "+example)
			b.WriteString(line + "\n")
		}
	}

	return b.String()
}

// viewFileNameTemplateSelect рисует список пресетов шаблонов имён файлов.
func (m Model) viewFileNameTemplateSelect() string {
	var b strings.Builder

	b.WriteString(highlightStyle.Render("Выберите формат имён файлов:"))
	b.WriteString("\n\n")

	for i, preset := range fileNamePresets {
		cursor := "  "
		if m.settings.fileNameCursor == i {
			cursor = highlightStyle.Render("▸ ")
		}

		if preset.label == "Свой формат…" {
			b.WriteString(cursor + preset.label + "\n")
		} else {
			labelCol := templateLabelStyle.Render(cursor + preset.label)
			line := lipgloss.JoinHorizontal(lipgloss.Top, labelCol, " → "+preset.value)
			b.WriteString(line + "\n")
		}
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// GetSettingBool возвращает значение boolean-настройки по ключу.
func (m Model) GetSettingBool(key string) bool {
	for _, s := range m.settings.items {
		if s.key == key && s.stype == settingTypeBool {
			return s.boolValue
		}
	}
	return false
}

// GetSettingString возвращает значение текстовой настройки по ключу.
func (m Model) GetSettingString(key string) string {
	for _, s := range m.settings.items {
		if s.key == key && (s.stype == settingTypeText || s.stype == settingTypeChoice) {
			return s.stringValue
		}
	}
	return ""
}
