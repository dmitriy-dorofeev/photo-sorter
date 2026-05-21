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
	if len(m.settings.items) != 17 {
		t.Errorf("expected 17 settings, got %d", len(m.settings.items))
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
	if m.settings.items[2].key != "sort_mode" {
		t.Errorf("expected third setting key 'sort_mode', got %s", m.settings.items[2].key)
	}
	if m.settings.items[2].AsString() != config.DefaultSortMode {
		t.Errorf("expected default sort_mode %q, got %q", config.DefaultSortMode, m.settings.items[2].AsString())
	}
	if m.settings.items[3].key != "face_similarity" {
		t.Errorf("expected fourth setting key 'face_similarity', got %s", m.settings.items[3].key)
	}
	if m.settings.items[4].key != "live_photos" {
		t.Errorf("expected fifth setting key 'live_photos', got %s", m.settings.items[4].key)
	}
	if m.settings.items[4].AsBool() != config.DefaultLivePhotos {
		t.Errorf("expected default live_photos %v, got %v", config.DefaultLivePhotos, m.settings.items[4].AsBool())
	}
	if m.settings.items[5].key != "cluster_raw_jpeg" {
		t.Errorf("expected sixth setting key 'cluster_raw_jpeg', got %s", m.settings.items[5].key)
	}
	if m.settings.items[5].AsBool() != config.DefaultClusterRawJPEG {
		t.Errorf("expected default cluster_raw_jpeg %v, got %v", config.DefaultClusterRawJPEG, m.settings.items[5].AsBool())
	}
	if m.settings.items[8].key != "write_exif" {
		t.Errorf("expected ninth setting key 'write_exif', got %s", m.settings.items[8].key)
	}
	if m.settings.items[8].AsBool() != config.DefaultWriteExif {
		t.Errorf("expected default write_exif %v, got %v", config.DefaultWriteExif, m.settings.items[8].AsBool())
	}
	if m.settings.items[9].key != "write_spotlight" {
		t.Errorf("expected tenth setting key 'write_spotlight', got %s", m.settings.items[9].key)
	}
	if m.settings.items[9].AsBool() != config.DefaultWriteSpotlight {
		t.Errorf("expected default write_spotlight %v, got %v", config.DefaultWriteSpotlight, m.settings.items[9].AsBool())
	}
	if m.settings.items[10].key != "notify" {
		t.Errorf("expected eleventh setting key 'notify', got %s", m.settings.items[10].key)
	}
	if m.settings.items[10].AsBool() != config.DefaultNotify {
		t.Errorf("expected default notify %v, got %v", config.DefaultNotify, m.settings.items[10].AsBool())
	}
	if m.settings.items[11].key != "skip_sorted" {
		t.Errorf("expected twelfth setting key 'skip_sorted', got %s", m.settings.items[11].key)
	}
	if !m.settings.items[11].AsBool() {
		t.Errorf("expected default skip_sorted true, got %v", m.settings.items[11].AsBool())
	}
	if m.settings.items[12].key != "dup_strategy" {
		t.Errorf("expected thirteenth setting key 'dup_strategy', got %s", m.settings.items[12].key)
	}
	if m.settings.items[12].AsString() != config.DefaultDupStrategy {
		t.Errorf("expected default dup_strategy %q, got %q", config.DefaultDupStrategy, m.settings.items[12].AsString())
	}
	if m.settings.items[13].key != "collision_strategy" {
		t.Errorf("expected fourteenth setting key 'collision_strategy', got %s", m.settings.items[13].key)
	}
	if m.settings.items[13].AsString() != config.DefaultCollisionStrategy {
		t.Errorf("expected default collision_strategy %q, got %q", config.DefaultCollisionStrategy, m.settings.items[13].AsString())
	}
	if m.copyProgress == nil || m.copyTotal == nil {
		t.Error("expected copyProgress and copyTotal to be initialized")
	}
	if m.theme == nil {
		t.Error("expected theme to be initialized")
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

	// Move to quick start screen
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = newM.(Model)
	if m.screen != ScreenQuickStart {
		t.Errorf("expected screen QuickStart, got %d", m.screen)
	}

	// Move to settings screen from quick start
	m.quickStart.cursor = 1
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(Model)
	if m.screen != ScreenSettings {
		t.Errorf("expected screen Settings, got %d", m.screen)
	}

	// Back to quick start
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = newM.(Model)
	if m.screen != ScreenQuickStart {
		t.Errorf("expected screen QuickStart after back, got %d", m.screen)
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

	// Перемещаем курсор на write_exif (позиция 8)
	m.settings.cursor = 8

	// Переключаем настройку
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = newM.(Model)
	if m.settings.items[8].key != "write_exif" {
		t.Fatalf("expected cursor on write_exif, got %s", m.settings.items[m.settings.cursor].key)
	}
	// Переключаем значение
	if !m.GetSettingBool("write_exif") {
		t.Error("expected write_exif to be true after toggle")
	}
}

// ---------------------------------------------------------------------------
// Тесты создания папки и выбора текущей директории в target screen
// ---------------------------------------------------------------------------

func TestTargetCreateFolder(t *testing.T) {
	tmp := t.TempDir()

	m := NewModel("test")
	m.screen = ScreenTarget
	m.target.currentDir = tmp
	items, err := loadDirItems(tmp)
	if err != nil {
		t.Fatalf("loadDirItems: %v", err)
	}
	m.target.items = items

	// Нажимаем 'n' — входим в режим создания
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = newM.(Model)
	if !m.target.creating {
		t.Fatal("expected creating to be true")
	}
	if cmd == nil {
		t.Error("expected textinput.Blink command")
	}

	// Вводим имя папки и нажимаем Enter
	m.target.input.SetValue("new-folder")
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(Model)

	if m.target.creating {
		t.Error("expected creating to be false after successful creation")
	}
	if m.target.createErr != "" {
		t.Errorf("unexpected createErr: %s", m.target.createErr)
	}

	// Проверяем что папка создана на диске
	newPath := filepath.Join(tmp, "new-folder")
	info, err := os.Stat(newPath)
	if err != nil {
		t.Fatalf("expected folder to be created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected created path to be a directory")
	}

	// Проверяем что папка появилась в списке и курсор на ней
	found := false
	for i, item := range m.target.items {
		if item.name == "new-folder" {
			found = true
			if m.target.cursor != i {
				t.Errorf("expected cursor on new folder (idx %d), got %d", i, m.target.cursor)
			}
			break
		}
	}
	if !found {
		t.Error("expected new folder to appear in items")
	}
}

func TestTargetCreateFolderError(t *testing.T) {
	tmp := t.TempDir()

	m := NewModel("test")
	m.screen = ScreenTarget
	m.target.currentDir = tmp
	items, err := loadDirItems(tmp)
	if err != nil {
		t.Fatalf("loadDirItems: %v", err)
	}
	m.target.items = items

	// Входим в режим создания
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = newM.(Model)

	// Пустое имя
	m.target.input.SetValue("   ")
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(Model)
	if m.target.createErr == "" {
		t.Error("expected createErr for empty folder name")
	}
	if !m.target.creating {
		t.Error("expected still in creating mode after error")
	}

	// Запрещённые символы
	m.target.input.SetValue("foo/bar")
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(Model)
	if m.target.createErr == "" {
		t.Error("expected createErr for invalid folder name")
	}

	// Убеждаемся что папка не создана
	badPath := filepath.Join(tmp, "foo")
	if _, err := os.Stat(badPath); !os.IsNotExist(err) {
		t.Error("expected no folder to be created for invalid name")
	}

	// Отмена через Esc
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newM.(Model)
	if m.target.creating {
		t.Error("expected creating to be false after Esc")
	}
}

func TestTargetSelectCurrentDir(t *testing.T) {
	tmp := t.TempDir()

	m := NewModel("test")
	m.screen = ScreenTarget
	m.target.currentDir = tmp
	items, err := loadDirItems(tmp)
	if err != nil {
		t.Fatalf("loadDirItems: %v", err)
	}
	m.target.items = items

	// Нажимаем 'c' — выбираем текущую папку
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = newM.(Model)

	if m.Target != tmp {
		t.Errorf("expected Target to be %q, got %q", tmp, m.Target)
	}

	// Проверяем что можно перейти дальше
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = newM.(Model)
	if m.screen != ScreenQuickStart {
		t.Errorf("expected screen QuickStart, got %d", m.screen)
	}
}

func TestTargetSelectCurrentDirAndCreate(t *testing.T) {
	tmp := t.TempDir()

	m := NewModel("test")
	m.screen = ScreenTarget
	m.target.currentDir = tmp
	items, err := loadDirItems(tmp)
	if err != nil {
		t.Fatalf("loadDirItems: %v", err)
	}
	m.target.items = items

	// Выбираем текущую папку
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = newM.(Model)
	if m.Target != tmp {
		t.Fatalf("expected Target %q, got %q", tmp, m.Target)
	}

	// Создаём подпапку — Target не должен измениться
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = newM.(Model)
	m.target.input.SetValue("subfolder")
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(Model)

	if m.Target != tmp {
		t.Errorf("expected Target to remain %q after creating subfolder, got %q", tmp, m.Target)
	}

	// Проверяем что подпапка создана
	subPath := filepath.Join(tmp, "subfolder")
	if info, err := os.Stat(subPath); err != nil || !info.IsDir() {
		t.Error("expected subfolder to be created")
	}
}

// ---------------------------------------------------------------------------
// Тесты экрана быстрого старта (QuickStart)
// ---------------------------------------------------------------------------

func TestQuickStartScreen(t *testing.T) {
	m := NewModel("test")
	m.screen = ScreenQuickStart

	if m.quickStart.cursor != 0 {
		t.Errorf("expected quickStart cursor 0, got %d", m.quickStart.cursor)
	}

	// Переключение вниз
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newM.(Model)
	if m.quickStart.cursor != 1 {
		t.Errorf("expected quickStart cursor 1 after Down, got %d", m.quickStart.cursor)
	}

	// Переключение вниз — не выходит за границу
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newM.(Model)
	if m.quickStart.cursor != 1 {
		t.Errorf("expected quickStart cursor to stay 1, got %d", m.quickStart.cursor)
	}

	// Переключение вверх
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newM.(Model)
	if m.quickStart.cursor != 0 {
		t.Errorf("expected quickStart cursor 0 after Up, got %d", m.quickStart.cursor)
	}

	// Назад на Target
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = newM.(Model)
	if m.screen != ScreenTarget {
		t.Errorf("expected screen Target, got %d", m.screen)
	}
}

func TestQuickStartToSettings(t *testing.T) {
	m := NewModel("test")
	m.screen = ScreenQuickStart
	m.quickStart.cursor = 1

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(Model)
	if m.screen != ScreenSettings {
		t.Errorf("expected screen Settings, got %d", m.screen)
	}

	// Проверяем что ← возвращает на QuickStart
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = newM.(Model)
	if m.screen != ScreenQuickStart {
		t.Errorf("expected screen QuickStart after back from Settings, got %d", m.screen)
	}
}

func TestQuickStartToScan(t *testing.T) {
	tmp := t.TempDir()
	photos := filepath.Join(tmp, "photos")
	mustMkdir(t, photos)

	m := NewModel("test")
	m.screen = ScreenQuickStart
	m.Sources = []string{photos}
	m.Target = tmp
	m.quickStart.cursor = 0

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(Model)
	if m.screen != ScreenScan {
		t.Errorf("expected screen Scan, got %d", m.screen)
	}
	if !m.scan.running {
		t.Error("expected scan.running to be true")
	}
	if cmd == nil {
		t.Error("expected non-nil command (scan start)")
	}
}
