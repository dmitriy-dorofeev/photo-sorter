package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

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
	cursor  int
	items   []setting
	editing bool
	input   textinput.Model
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
				help:  "Go time layout для имени папок",
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
		},
		input: ti,
	}
}

func (s settingsModel) Init() tea.Cmd {
	return nil
}

func (m Model) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.settings.editing {
		return m.updateSettingsEditing(msg)
	}
	return m.updateSettingsNav(msg)
}

func (m Model) updateSettingsEditing(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.settings.editing = false
			m.settings.input.Blur()
			// сохраняем значение
			m.settings.items[m.settings.cursor].value = m.settings.input.Value()
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
		}

		switch msg.String() {
		case " ", "enter":
			item := &m.settings.items[m.settings.cursor]
			switch item.stype {
			case settingTypeBool:
				item.value = !item.value.(bool)
			case settingTypeText:
				m.settings.editing = true
				m.settings.input.SetValue(item.value.(string))
				m.settings.input.Focus()
				return m, textinput.Blink
			}
			return m, nil

		case "b":
			m.screen = ScreenTarget
			return m, nil

		case "n":
			m.screen = ScreenScan
			return m, nil
		}
	}

	return m, nil
}

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

	// Список настроек
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
				valueStr = highlightStyle.Render(item.value.(string))
			}
		}

		b.WriteString(fmt.Sprintf("%s%-25s %s\n", cursor, item.label+":", valueStr))
		if m.settings.cursor == i && !m.settings.editing {
			b.WriteString(helpStyle.Render(fmt.Sprintf("     %s\n", item.help)))
		}
	}

	b.WriteString("\n")

	if m.settings.editing {
		b.WriteString(helpStyle.Render("enter — сохранить • esc — отменить"))
	} else {
		b.WriteString(helpStyle.Render("↑/↓ — выбрать • enter/пробел — изменить • b — назад • n — продолжить • esc — выход"))
	}

	return b.String()
}

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
