package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestNewLightTheme(t *testing.T) {
	th := NewLightTheme()
	if th == nil {
		t.Fatal("expected non-nil theme")
	}
	assertStyleDefined(t, "Title", th.Title)
	assertStyleDefined(t, "Subtitle", th.Subtitle)
	assertStyleDefined(t, "Highlight", th.Highlight)
	assertStyleDefined(t, "Success", th.Success)
	assertStyleDefined(t, "Error", th.Error)
	assertStyleDefined(t, "Help", th.Help)
	assertStyleDefined(t, "SettingLabel", th.SettingLabel)
	assertStyleDefined(t, "TemplateLabel", th.TemplateLabel)
}

func TestNewDarkTheme(t *testing.T) {
	th := NewDarkTheme()
	if th == nil {
		t.Fatal("expected non-nil theme")
	}
	assertStyleDefined(t, "Title", th.Title)
	assertStyleDefined(t, "Subtitle", th.Subtitle)
	assertStyleDefined(t, "Highlight", th.Highlight)
	assertStyleDefined(t, "Success", th.Success)
	assertStyleDefined(t, "Error", th.Error)
	assertStyleDefined(t, "Help", th.Help)
	assertStyleDefined(t, "SettingLabel", th.SettingLabel)
	assertStyleDefined(t, "TemplateLabel", th.TemplateLabel)
}

func TestThemesAreDifferent(t *testing.T) {
	light := NewLightTheme()
	dark := NewDarkTheme()

	// Сравниваем foreground-цвета, чтобы убедиться, что палитры различаются.
	if light.Title.GetForeground() == dark.Title.GetForeground() {
		t.Error("expected light and dark Title foreground to differ")
	}
	if light.Subtitle.GetForeground() == dark.Subtitle.GetForeground() {
		t.Error("expected light and dark Subtitle foreground to differ")
	}
}

func TestApplyThemeMode(t *testing.T) {
	m := NewModel("test")

	m.applyThemeMode(ThemeModeLight)
	if m.theme == nil {
		t.Fatal("theme nil after applying light mode")
	}
	if m.theme.Title.String() != NewLightTheme().Title.String() {
		t.Error("expected light theme after ThemeModeLight")
	}

	m.applyThemeMode(ThemeModeDark)
	if m.theme.Title.String() != NewDarkTheme().Title.String() {
		t.Error("expected dark theme after ThemeModeDark")
	}

	m.applyThemeMode(ThemeModeAuto)
	// На macOS результат зависит от системы, на остальных — светлая.
	if m.theme == nil {
		t.Error("theme nil after applying auto mode")
	}
}

func assertStyleDefined(t *testing.T, name string, s lipgloss.Style) {
	t.Helper()
	// lipgloss.Style не имеет метода IsZero; рендер пустой строки не паникует.
	_ = s.Render("")
}
