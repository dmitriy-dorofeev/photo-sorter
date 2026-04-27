package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"photo-sorter/internal/sorter"
)

func (m Model) updatePreview(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyLeft:
			m.screen = ScreenScan
			return m, nil
		case tea.KeyRight, tea.KeyEnter:
			m.screen = ScreenCopy
			m.copy = newCopyModel()
			m.copy.running = true
			m.copyProgress.Store(0)
			m.copyTotal.Store(int64(len(m.entries)))
			return m, tea.Batch(copyTickCmd(m), m.startCopy())
		}
	}
	return m, nil
}

func (m Model) viewPreview() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(" photo-sorter "))
	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render("Шаг 5. Предпросмотр"))
	b.WriteString("\n\n")

	st := m.computeScanStats()
	b.WriteString(fmt.Sprintf(
		"Найдено файлов: %d  |  Дат определено: %d  |  Без даты: %d  |  Дублей: %d\n",
		st.total, st.withDate, st.unsorted, st.duplicates,
	))
	b.WriteString("\n")

	// Вычисляем лимиты списков из высоты терминала.
	maxDirs := 15
	maxSmall := 5
	if m.height > 0 {
		available := m.height - 12
		if available > 5 {
			maxDirs = available
			maxSmall = max(3, available/3)
		}
	}

	// Дерево папок
	dirs := m.previewDirs()
	if len(dirs) > 0 {
		b.WriteString(highlightStyle.Render("Целевая структура:") + "\n")
		shown := 0
		for _, d := range dirs {
			if shown >= maxDirs {
				b.WriteString(fmt.Sprintf("  … и ещё %d папок\n", len(dirs)-shown))
				break
			}
			rel, _ := filepath.Rel(m.Target, d)
			b.WriteString(fmt.Sprintf("  📁 %s (%d файлов)\n", rel, m.dirFileCount(d)))
			shown++
		}
		b.WriteString("\n")
	}

	// Дубли
	if len(m.duplicates) > 0 {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Дубликаты (%d групп):", len(m.duplicates))) + "\n")
		for i, dup := range m.duplicates {
			if i >= maxSmall {
				b.WriteString(fmt.Sprintf("  … и ещё %d групп\n", len(m.duplicates)-i))
				break
			}
			b.WriteString(fmt.Sprintf("  • %s → %d дублей\n", dup.Original.Name, len(dup.Duplicates)))
		}
		b.WriteString("\n")
	}

	// Unsorted
	unsorted := m.unsortedFiles()
	if len(unsorted) > 0 {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Без даты — unsorted/ (%d файлов):", len(unsorted))) + "\n")
		for i, f := range unsorted {
			if i >= maxSmall {
				b.WriteString(fmt.Sprintf("  … и ещё %d\n", len(unsorted)-i))
				break
			}
			b.WriteString(fmt.Sprintf("  • %s\n", f))
		}
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("← — назад • enter — запустить копирование • esc — выход"))
	return b.String()
}

// buildPreviewCache строит кэш директорий и счётчиков файлов.
// Вызывается один раз при получении entries (в updateScan).
func (m Model) buildPreviewCache() Model {
	if len(m.entries) == 0 {
		m.previewDirCache = nil
		m.previewCountCache = nil
		return m
	}
	dirSet := make(map[string]struct{})
	countMap := make(map[string]int)
	for _, e := range m.entries {
		if e.Skip {
			continue
		}
		dir := filepath.Dir(e.Target)
		dirSet[dir] = struct{}{}
		countMap[dir]++
	}
	dirs := make([]string, 0, len(dirSet))
	for d := range dirSet {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	m.previewDirCache = dirs
	m.previewCountCache = countMap
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
			files = append(files, e.Source.Name)
		}
	}
	return files
}
