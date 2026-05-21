package tui

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"photo-sorter/internal/sorter"
)

func (m Model) updatePreview(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Если мы в режиме редактирования alias'а — передаём ввод в textinput
	if m.faceAliasRenaming {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyEsc:
				m.faceAliasRenaming = false
				m.faceAliasInput.Blur()
				return m, nil
			case tea.KeyEnter:
				newName := strings.TrimSpace(m.faceAliasInput.Value())
				if newName != "" {
					m = m.renameFaceAlias(m.faceAliasList[m.faceAliasCursor], newName)
				}
				m.faceAliasRenaming = false
				m.faceAliasInput.Blur()
				return m, nil
			default:
				var cmd tea.Cmd
				m.faceAliasInput, cmd = m.faceAliasInput.Update(msg)
				return m, cmd
			}
		}
		return m, nil
	}

	// Режим просмотра примера файла
	if m.faceAliasViewing {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyEsc:
				m.faceAliasViewing = false
				return m, nil
			case tea.KeyEnter:
				alias := m.faceAliasList[m.faceAliasCursor]
				path := m.faceAliasFullSamples[alias]
				if path != "" {
					return m, openFileCmd(path)
				}
			default:
				if msg.String() == "v" {
					m.faceAliasViewing = false
					return m, nil
				}
			}
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyLeft:
			m.faceAliasRenaming = false
			m.faceAliasViewing = false
			m.faceAliasInput.Blur()
			m.screen = ScreenScan
			return m, nil
		case tea.KeyRight, tea.KeyEnter:
			if !m.faceAliasRenaming && !m.faceAliasViewing {
				m.screen = ScreenCopy
				m.copy = newCopyModel()
				m.copy.running = true
				m.copyProgress.Store(0)
				m.copyTotal.Store(int64(len(m.entries)))
				m, cmd := m.startCopy()
				return m, tea.Batch(copyTickCmd(m), cmd)
			}
		case tea.KeyUp:
			if m.isFaceMode() && len(m.faceAliasList) > 0 {
				if m.faceAliasCursor > 0 {
					m.faceAliasCursor--
				}
			}
			return m, nil
		case tea.KeyDown:
			if m.isFaceMode() && len(m.faceAliasList) > 0 {
				if m.faceAliasCursor < len(m.faceAliasList)-1 {
					m.faceAliasCursor++
				}
			}
			return m, nil
		default:
			if m.isFaceMode() {
				switch msg.String() {
				case "r":
					if len(m.faceAliasList) > 0 && m.faceAliasCursor < len(m.faceAliasList) {
						m.faceAliasRenaming = true
						m.faceAliasInput = textinput.New()
						m.faceAliasInput.Placeholder = "Новое имя"
						m.faceAliasInput.SetValue(m.faceAliasList[m.faceAliasCursor])
						m.faceAliasInput.Focus()
						m.faceAliasInput.CharLimit = 40
						return m, textinput.Blink
					}
				case "v":
					if len(m.faceAliasList) > 0 && m.faceAliasCursor < len(m.faceAliasList) {
						alias := m.faceAliasList[m.faceAliasCursor]
						if m.faceAliasFullSamples[alias] != "" {
							m.faceAliasViewing = true
							return m, nil
						}
					}
				}
			}
			return m, nil
		}
	}
	return m, nil
}

func (m Model) viewPreview() string {
	if m.isFaceMode() {
		return m.viewFacePreview()
	}
	return m.viewDatePreview()
}

// viewDatePreview — стандартный предпросмотр для режима сортировки по датам.
func (m Model) viewDatePreview() string {
	var b strings.Builder

	b.WriteString(m.theme.Title.Render(" photo-sorter "))
	b.WriteString("\n\n")
	b.WriteString(m.theme.Subtitle.Render("Шаг 5. Предпросмотр"))
	b.WriteString("\n\n")

	st := m.computeScanStats()
	b.WriteString(fmt.Sprintf(
		"Найдено файлов: %d  |  Дат определено: %d  |  Без даты: %d  |  Дублей: %d\n",
		st.total, st.withDate, st.unsorted, st.duplicates,
	))
	b.WriteString("\n")

	maxDirs := 15
	maxSmall := 5
	if m.height > 0 {
		available := m.height - 12
		if available > 5 {
			maxDirs = available
			maxSmall = max(3, available/3)
		}
	}

	dirs := m.previewDirs()
	if len(dirs) > 0 {
		b.WriteString(m.theme.Highlight.Render("Целевая структура:") + "\n")
		shown := 0
		for _, d := range dirs {
			if shown >= maxDirs {
				b.WriteString(fmt.Sprintf("  … и ещё %d папок\n", len(dirs)-shown))
				break
			}
			rel, err := filepath.Rel(m.Target, d)
			if err != nil {
				rel = d
			}
			b.WriteString(fmt.Sprintf("  📁 %s (%d файлов)\n", rel, m.dirFileCount(d)))
			shown++
		}
		b.WriteString("\n")
	}

	if len(m.duplicates) > 0 {
		b.WriteString(m.theme.Error.Render(fmt.Sprintf("Дубликаты (%d групп):", len(m.duplicates))) + "\n")
		for i, dup := range m.duplicates {
			if i >= maxSmall {
				b.WriteString(fmt.Sprintf("  … и ещё %d групп\n", len(m.duplicates)-i))
				break
			}
			b.WriteString(fmt.Sprintf("  • %s → %d дублей\n", dup.Original.Name, len(dup.Duplicates)))
		}
		b.WriteString("\n")
	}

	unsorted := m.unsortedFiles()
	if len(unsorted) > 0 {
		b.WriteString(m.theme.Error.Render(fmt.Sprintf("Без даты — unsorted/ (%d файлов):", len(unsorted))) + "\n")
		for i, f := range unsorted {
			if i >= maxSmall {
				b.WriteString(fmt.Sprintf("  … и ещё %d\n", len(unsorted)-i))
				break
			}
			b.WriteString(fmt.Sprintf("  • %s\n", f))
		}
		b.WriteString("\n")
	}

	b.WriteString(m.theme.Help.Render("← — назад • enter — запустить копирование • esc — выход"))
	return b.String()
}

// viewFacePreview — предпросмотр для режима сортировки по лицам.
func (m Model) viewFacePreview() string {
	var b strings.Builder

	b.WriteString(m.theme.Title.Render(" photo-sorter "))
	b.WriteString("\n\n")
	b.WriteString(m.theme.Subtitle.Render("Шаг 5. Предпросмотр (face-режим)"))
	b.WriteString("\n\n")

	st := m.computeScanStats()
	b.WriteString(fmt.Sprintf(
		"Найдено файлов: %d  |  Дублей: %d\n",
		st.total, st.duplicates,
	))
	b.WriteString("\n")

	if len(m.faceAliasList) == 0 {
		b.WriteString("Нет найденных лиц.\n")
		b.WriteString(m.theme.Help.Render("← — назад • enter — запустить копирование • esc — выход"))
		return b.String()
	}

	maxItems := 20
	if m.height > 0 {
		available := m.height - 14
		if available > 5 {
			maxItems = available
		}
	}

	b.WriteString(m.theme.Highlight.Render("Найденные люди:") + "\n")

	start := 0
	end := len(m.faceAliasList)
	if m.faceAliasCursor >= maxItems {
		start = m.faceAliasCursor - maxItems/2
		if start < 0 {
			start = 0
		}
	}
	if end-start > maxItems {
		end = start + maxItems
	}

	for i := start; i < end; i++ {
		alias := m.faceAliasList[i]
		count := 0
		for _, e := range m.entries {
			if !e.Skip && faceDirFromTarget(e.Target) == alias {
				count++
			}
		}
		prefix := "  "
		if i == m.faceAliasCursor {
			prefix = m.theme.Highlight.Render("▶ ")
		}
		b.WriteString(fmt.Sprintf("%s%s (%d файлов)", prefix, alias, count))
		if samples := m.faceAliasSamples[alias]; len(samples) > 0 {
			b.WriteString(fmt.Sprintf(" — %s", strings.Join(samples, ", ")))
		}
		b.WriteString("\n")
	}

	if end < len(m.faceAliasList) {
		b.WriteString(fmt.Sprintf("  … и ещё %d\n", len(m.faceAliasList)-end))
	}
	b.WriteString("\n")

	switch {
	case m.faceAliasViewing:
		alias := m.faceAliasList[m.faceAliasCursor]
		path := m.faceAliasFullSamples[alias]
		b.WriteString(m.theme.Highlight.Render("Пример файла:") + "\n")
		b.WriteString(fmt.Sprintf("  %s\n", path))
		b.WriteString("\n")
		b.WriteString(m.theme.Help.Render("enter — открыть в просмотрщике • v/esc — закрыть"))
	case m.faceAliasRenaming:
		b.WriteString(m.theme.Highlight.Render("Переименование: ") + m.faceAliasInput.View() + "\n")
		b.WriteString(m.theme.Help.Render("enter — подтвердить • esc — отменить"))
	default:
		b.WriteString(m.theme.Help.Render("↑/↓ — выбрать • v — пример • r — переименовать • enter — запустить • ← — назад • esc — выход"))
	}
	return b.String()
}

// renameFaceAlias переименовывает alias во всех записях.
func (m Model) renameFaceAlias(oldName, newName string) Model {
	if oldName == newName {
		return m
	}

	// 1. Обновляем entries: меняем папку в Target
	for i := range m.entries {
		if m.entries[i].Skip {
			continue
		}
		dir := faceDirFromTarget(m.entries[i].Target)
		if dir == oldName {
			m.entries[i].Target = filepath.Join(m.Target, newName, filepath.Base(m.entries[i].Target))
		}
	}

	// 2. Обновляем faceAliases: все ключи со старым значением → новое
	for k, v := range m.faceAliases {
		if v == oldName {
			m.faceAliases[k] = newName
		}
	}

	// 3. Перестраиваем кэш
	m = buildPreviewCache(m)
	return m
}

// faceDirFromTarget извлекает имя папки alias'а из абсолютного target-пути.
// Ожидается: <targetRoot>/<alias>/<filename>.
func faceDirFromTarget(target string) string {
	dir := filepath.Dir(target)
	return filepath.Base(dir)
}

// isFaceMode возвращает true, если текущий режим сортировки — по лицам.
func (m Model) isFaceMode() bool {
	return m.GetSettingString("sort_mode") == "face"
}

// openFileCmd возвращает команду для открытия файла в системном просмотрщике.
func openFileCmd(path string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", path)
		case "linux":
			cmd = exec.Command("xdg-open", path)
		case "windows":
			cmd = exec.Command("cmd", "/c", "start", "", path)
		default:
			return nil
		}
		_ = cmd.Start() // #nosec G204 — path приходит из внутренних данных
		return nil
	}
}

// buildPreviewCache строит кэш директорий, счётчиков и примеров файлов.
func buildPreviewCache(m Model) Model {
	if len(m.entries) == 0 {
		m.previewDirCache = nil
		m.previewCountCache = nil
		m.previewFileCache = nil
		m.faceAliasList = nil
		m.faceAliasSamples = nil
		m.faceAliasFullSamples = nil
		return m
	}

	dirSet := make(map[string]struct{})
	countMap := make(map[string]int)
	fileMap := make(map[string][]string)
	aliasSet := make(map[string]struct{})
	aliasSamples := make(map[string][]string)
	aliasFullSamples := make(map[string]string)

	for _, e := range m.entries {
		if e.Skip {
			continue
		}
		dir := filepath.Dir(e.Target)
		dirSet[dir] = struct{}{}
		countMap[dir]++
		if len(fileMap[dir]) < 3 {
			fileMap[dir] = append(fileMap[dir], filepath.Base(e.Target))
		}
		if m.isFaceMode() {
			alias := faceDirFromTarget(e.Target)
			aliasSet[alias] = struct{}{}
			if len(aliasSamples[alias]) < 3 {
				aliasSamples[alias] = append(aliasSamples[alias], filepath.Base(e.Target))
			}
			if aliasFullSamples[alias] == "" {
				aliasFullSamples[alias] = e.Source.Path
			}
		}
	}

	dirs := make([]string, 0, len(dirSet))
	for d := range dirSet {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	m.previewDirCache = dirs
	m.previewCountCache = countMap
	m.previewFileCache = fileMap

	if m.isFaceMode() {
		aliases := make([]string, 0, len(aliasSet))
		for a := range aliasSet {
			aliases = append(aliases, a)
		}
		sort.Strings(aliases)
		m.faceAliasList = aliases
		m.faceAliasSamples = aliasSamples
		m.faceAliasFullSamples = aliasFullSamples
		if m.faceAliasCursor >= len(aliases) {
			m.faceAliasCursor = 0
		}
	}

	return m
}

func (m Model) previewDirs() []string {
	return m.previewDirCache
}

func (m Model) dirFileCount(dir string) int {
	return m.previewCountCache[dir]
}

func (m Model) unsortedFiles() []string {
	var files []string
	for _, e := range m.entries {
		if !e.Skip && sorter.IsUnsorted(e.Target) {
			files = append(files, filepath.Base(e.Target))
		}
	}
	return files
}
