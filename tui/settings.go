package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Пресеты шаблонов папок (понятные пользователю метки → Go time layouts)
// ---------------------------------------------------------------------------

type templatePreset struct {
	label string // например "YYYY/MM/DD"
	value string // Go layout, например "2006/01/02"
}

var templatePresets = []templatePreset{
	{label: "YYYY/MM/DD", value: "2006/01/02"},
	{label: "YYYY-MM-DD", value: "2006-01-02"},
	{label: "YYYY/MM", value: "2006/01"},
	{label: "YYYY", value: "2006"},
	{label: "Свой формат…", value: ""},
}

// findPreset ищет пресет по Go-значению. Возвращает индекс и found.
func findPreset(value string) (int, bool) {
	for i, p := range templatePresets {
		if p.value == value {
			return i, true
		}
	}
	return len(templatePresets) - 1, false // указываем на "Свой формат…"
}

// formatTemplateDisplay возвращает человекочитаемое представление шаблона.
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
// Настройки
// ---------------------------------------------------------------------------

type settingType int

const (
	settingTypeText settingType = iota
	settingTypeBool
)

type setting struct {
	label string
	key   string
	help  string
	value interface{} // string или bool
	stype settingType
}

// settingsModel — состояние экрана настроек.
type settingsModel struct {
	cursor         int
	items          []setting
	editing        bool // true → редактируем custom через textinput
	templateSelect bool // true → показываем список пресетов
	templateCursor int  // курсор внутри списка пресетов
	input          textinput.Model
}

func newSettingsModel() settingsModel {
	ti := textinput.New()
	ti.Placeholder = "2006/01/02"
	ti.Width = 30

	return settingsModel{
		cursor: 0,
		items: []setting{
			{
				label: "Шаблон папок",
				key:   "template",
				help:  "Формат именования папок по дате",
				value: "2006/01/02",
				stype: settingTypeText,
			},
			{
				label: "Группировать Live Photos",
				key:   "live_photos",
				help:  "Копировать .mov рядом с .heic",
				value: true,
				stype: settingTypeBool,
			},
			{
				label: "Включать видео",
				key:   "include_video",
				help:  "Обрабатывать видеофайлы",
				value: true,
				stype: settingTypeBool,
			},
			{
				label: "Dry-run (пробный прогон)",
				key:   "dry_run",
				help:  "Показать результат без копирования",
				value: true,
				stype: settingTypeBool,
			},
			{
				label: "Использовать дату изменения",
				key:   "use_mtime",
				help:  "Если нет EXIF/имени, использовать ModTime файла",
				value: false,
				stype: settingTypeBool,
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
	case m.settings.editing:
		return m.updateSettingsEditing(msg)
	default:
		return m.updateSettingsNav(msg)
	}
}

// Режим выбора пресета шаблона.
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
			if preset.label == "Свой формат…" {
				// Переходим в ручной ввод
				m.settings.templateSelect = false
				m.settings.editing = true
				m.settings.input.SetValue(m.settings.items[m.settings.cursor].value.(string))
				m.settings.input.Focus()
				return m, textinput.Blink
			}
			// Выбрали готовый пресет
			m.settings.items[m.settings.cursor].value = preset.value
			m.settings.templateSelect = false
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
			m.settings.items[m.settings.cursor].value = m.settings.input.Value()
			return m, nil
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
			return m, tea.Batch(scanTickCmd(), m.startScan())
		}

		switch msg.String() {
		case " ", "enter":
			item := &m.settings.items[m.settings.cursor]
			switch item.stype {
			case settingTypeBool:
				item.value = !item.value.(bool)
			case settingTypeText:
				// Открываем выбор пресета
				m.settings.templateSelect = true
				idx, _ := findPreset(item.value.(string))
				m.settings.templateCursor = idx
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
	b.WriteString(highlightStyle.Render("Источник: "))
	b.WriteString(m.Source + "\n")
	b.WriteString(highlightStyle.Render("Цель: "))
	b.WriteString(m.Target + "\n")
	b.WriteString("\n")

	// Если выбираем пресет шаблона — рисуем модальный список поверх
	if m.settings.templateSelect {
		b.WriteString(m.viewTemplateSelect())
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("↑/↓ — выбрать • enter — применить • esc — отмена"))
		return b.String()
	}

	// Список настроек
	labelWidth := 28
	for i, item := range m.settings.items {
		cursor := "  "
		if m.settings.cursor == i {
			cursor = highlightStyle.Render("▸ ")
		}

		var valueStr string
		switch item.stype {
		case settingTypeBool:
			if item.value.(bool) {
				valueStr = successStyle.Render("✓ да")
			} else {
				valueStr = errorStyle.Render("✗ нет")
			}
		case settingTypeText:
			if m.settings.editing && m.settings.cursor == i {
				valueStr = m.settings.input.View()
			} else {
				valueStr = highlightStyle.Render(formatTemplateDisplay(item.value.(string)))
			}
		}

		labelCol := lipgloss.NewStyle().Width(labelWidth).Render(cursor + item.label + ":")
		line := lipgloss.JoinHorizontal(lipgloss.Top, labelCol, valueStr)
		b.WriteString(line + "\n")

		if m.settings.cursor == i && !m.settings.editing {
			helpLine := helpStyle.Render("  " + item.help)
			b.WriteString(helpLine + "\n")
		}
	}

	b.WriteString("\n")

	if m.settings.editing {
		b.WriteString(helpStyle.Render("enter — сохранить • esc — отменить"))
	} else {
		b.WriteString(helpStyle.Render("↑/↓ — выбрать • enter/пробел — изменить • ← — назад • → — продолжить • esc — выход"))
	}

	return b.String()
}

// viewTemplateSelect рисует список пресетов шаблонов.
func (m Model) viewTemplateSelect() string {
	var b strings.Builder

	b.WriteString(highlightStyle.Render("Выберите формат папок:"))
	b.WriteString("\n\n")

	labelWidth := 16
	for i, preset := range templatePresets {
		cursor := "  "
		if m.settings.templateCursor == i {
			cursor = highlightStyle.Render("▸ ")
		}

		if preset.label == "Свой формат…" {
			b.WriteString(cursor + preset.label + "\n")
		} else {
			example := time.Now().Format(preset.value)
			labelCol := lipgloss.NewStyle().Width(labelWidth).Render(cursor + preset.label)
			line := lipgloss.JoinHorizontal(lipgloss.Top, labelCol, " → "+example)
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
			return s.value.(bool)
		}
	}
	return false
}

// GetSettingString возвращает значение текстовой настройки по ключу.
func (m Model) GetSettingString(key string) string {
	for _, s := range m.settings.items {
		if s.key == key && s.stype == settingTypeText {
			return s.value.(string)
		}
	}
	return ""
}
