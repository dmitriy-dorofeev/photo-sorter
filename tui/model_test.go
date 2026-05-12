package tui

import (
	"os"
	"path/filepath"
	"testing"

	"photo-sorter/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModel(t *testing.T) {
	m := NewModel("1.2.3")

	if m.screen != ScreenSources {
		t.Errorf("expected initial screen Sources, got %d", m.screen)
	}
	if m.version != "1.2.3" {
		t.Errorf("expected version 1.2.3, got %s", m.version)
	}
	if len(m.Sources) != 0 {
		t.Errorf("expected empty Sources, got %v", m.Sources)
	}
	if len(m.settings.items) != 8 {
		t.Errorf("expected 8 settings, got %d", len(m.settings.items))
	}
	if m.settings.cursor != 0 {
		t.Errorf("expected settings cursor 0, got %d", m.settings.cursor)
	}
	if m.settings.items[0].key != "template" {
		t.Errorf("expected first setting key 'template', got %s", m.settings.items[0].key)
	}
	if m.settings.items[0].AsString() != config.DefaultTemplate {
		t.Errorf("expected default template %q, got %q", config.DefaultTemplate, m.settings.items[0].AsString())
	}
	if m.settings.items[1].key != "file_name_template" {
		t.Errorf("expected second setting key 'file_name_template', got %s", m.settings.items[1].key)
	}
	if m.settings.items[1].AsString() != config.DefaultFileNameTemplate {
		t.Errorf("expected default file_name_template %q, got %q", config.DefaultFileNameTemplate, m.settings.items[1].AsString())
	}
	if m.settings.items[2].key != "live_photos" {
		t.Errorf("expected third setting key 'live_photos', got %s", m.settings.items[2].key)
	}
	if m.settings.items[2].AsBool() != config.DefaultLivePhotos {
		t.Errorf("expected default live_photos %v, got %v", config.DefaultLivePhotos, m.settings.items[2].AsBool())
	}
	if m.settings.items[5].key != "write_exif" {
		t.Errorf("expected sixth setting key 'write_exif', got %s", m.settings.items[5].key)
	}
	if m.settings.items[5].AsBool() != config.DefaultWriteExif {
		t.Errorf("expected default write_exif %v, got %v", config.DefaultWriteExif, m.settings.items[5].AsBool())
	}
	if m.settings.items[6].key != "dup_strategy" {
		t.Errorf("expected seventh setting key 'dup_strategy', got %s", m.settings.items[6].key)
	}
	if m.settings.items[6].AsString() != config.DefaultDupStrategy {
		t.Errorf("expected default dup_strategy %q, got %q", config.DefaultDupStrategy, m.settings.items[6].AsString())
	}
	if m.settings.items[7].key != "collision_strategy" {
		t.Errorf("expected eighth setting key 'collision_strategy', got %s", m.settings.items[7].key)
	}
	if m.settings.items[7].AsString() != config.DefaultCollisionStrategy {
		t.Errorf("expected default collision_strategy %q, got %q", config.DefaultCollisionStrategy, m.settings.items[7].AsString())
	}
	if m.copyProgress == nil || m.copyTotal == nil {
		t.Error("expected copyProgress and copyTotal to be initialized")
	}
}

func TestScreenTransitions(t *testing.T) {
	tmp := t.TempDir()
	photos := filepath.Join(tmp, "photos")
	output := filepath.Join(tmp, "output")
	mustMkdir(t, photos)
	mustMkdir(t, output)

	m := NewModel("test")

	// Prepare source browser
	m.sources.currentDir = tmp
	items, err := loadDirItems(tmp)
	if err != nil {
		t.Fatalf("loadDirItems: %v", err)
	}
	m.sources.items = items
	m.sources.cursor = 1 // first real dir (output or photos, sorted alphabetically)

	// Select source with space
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = newM.(Model)
	if len(m.Sources) == 0 {
		t.Fatal("expected at least one source selected")
	}

	// Move to target screen
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = newM.(Model)
	if m.screen != ScreenTarget {
		t.Errorf("expected screen Target, got %d", m.screen)
	}

	// Prepare target browser and select target
	m.target.currentDir = tmp
	items, err = loadDirItems(tmp)
	if err != nil {
		t.Fatalf("loadDirItems: %v", err)
	}
	m.target.items = items
	m.target.cursor = 1
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = newM.(Model)
	if m.Target == "" {
		t.Fatal("expected target to be selected")
	}

	// Move to settings screen
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = newM.(Model)
	if m.screen != ScreenSettings {
		t.Errorf("expected screen Settings, got %d", m.screen)
	}

	// Back to target
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = newM.(Model)
	if m.screen != ScreenTarget {
		t.Errorf("expected screen Target after back, got %d", m.screen)
	}

	// Back to sources
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = newM.(Model)
	if m.screen != ScreenSources {
		t.Errorf("expected screen Sources after second back, got %d", m.screen)
	}
}

func TestSettingsValidation(t *testing.T) {
	m := NewModel("test")
	m.screen = ScreenSettings

	// Open template preset selector
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = newM.(Model)
	if !m.settings.templateSelect {
		t.Fatal("expected templateSelect to be true")
	}

	// Choose custom preset (last item)
	m.settings.templateCursor = len(templatePresets) - 1
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(Model)
	if m.settings.templateSelect {
		t.Error("expected templateSelect to be closed")
	}
	if !m.settings.editing {
		t.Error("expected editing mode for custom template")
	}
	if cmd == nil {
		t.Error("expected textinput.Blink command")
	}

	// Type invalid template
	m.settings.input.SetValue("invalid!!!")
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(Model)
	if m.settings.editing {
		t.Error("expected editing mode to finish")
	}
	if got := m.GetSettingString("template"); got != "invalid!!!" {
		t.Errorf("expected template 'invalid!!!', got %q", got)
	}

	// formatTemplateDisplay must not panic for invalid layouts
	display := formatTemplateDisplay("invalid!!!")
	if display != "Свой формат (invalid!!!)" {
		t.Errorf("unexpected display for invalid template: got %q", display)
	}

	// Ensure GetSettingBool returns default for unknown key
	if m.GetSettingBool("nonexistent") {
		t.Error("expected false for unknown bool key")
	}
	if m.GetSettingString("nonexistent") != "" {
		t.Error("expected empty string for unknown string key")
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
}

func TestSettings_WriteExif(t *testing.T) {
	m := NewModel("test")
	m.screen = ScreenSettings

	if m.GetSettingBool("write_exif") {
		t.Error("expected write_exif to be false by default")
	}

	// Перемещаем курсор на write_exif (позиция 5)
	m.settings.cursor = 5

	// Переключаем настройку
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = newM.(Model)
	if m.settings.items[5].key != "write_exif" {
		t.Fatalf("expected cursor on write_exif, got %s", m.settings.items[m.settings.cursor].key)
	}
	// Переключаем значение
	if !m.GetSettingBool("write_exif") {
		t.Error("expected write_exif to be true after toggle")
	}
}
