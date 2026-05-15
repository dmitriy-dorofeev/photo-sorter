package tui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"photo-sorter/internal/collision"
	"photo-sorter/internal/copier"
	"photo-sorter/internal/logger"
	"photo-sorter/internal/notify"
	"photo-sorter/internal/state"

	tea "github.com/charmbracelet/bubbletea"
)

type copyTickMsg struct {
	current int
	total   int
}

type copyDoneMsg struct {
	stats copier.Stats
	err   error
}

type copyModel struct {
	width   int
	height  int
	running bool
	current int
	total   int
	done    bool
	errMsg  string
	stats   copier.Stats
	aborted bool   // true если пользователь нажал Esc
	logErr  string // ошибка создания лог-файла (предупреждение)
}

func newCopyModel() copyModel {
	return copyModel{}
}

func copyTickCmd(m Model) tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return copyTickMsg{
			current: int(m.copyProgress.Load()),
			total:   int(m.copyTotal.Load()),
		}
	})
}

func (m Model) startCopy() (Model, tea.Cmd) {
	// Отменяем предыдущее копирование, если оно ещё бежит.
	if m.copyCancel != nil {
		m.copyCancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.copyCancel = cancel

	dryRun := false

	return m, func() tea.Msg {
		strategy := collision.Strategy(m.GetSettingString("collision_strategy"))
		c := copier.New(dryRun, m.Target, strategy)
		c.WriteExif = m.GetSettingBool("write_exif") && m.exifToolPath != ""
		c.ExifToolPath = m.exifToolPath
		stats, err := c.Copy(ctx, m.entries, func(cur, tot int) {
			m.copyProgress.Store(int64(cur))
			m.copyTotal.Store(int64(tot))
		})
		return copyDoneMsg{stats: stats, err: err}
	}
}

func (m Model) updateCopy(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case copyTickMsg:
		if !m.copy.running {
			return m, nil
		}
		m.copy.current = msg.current
		m.copy.total = msg.total
		return m, copyTickCmd(m)

	case copyDoneMsg:
		if m.copy.aborted {
			return m, nil
		}
		m.copy.running = false
		m.copy.done = true
		m.copy.stats = msg.stats
		if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
			m.copy.errMsg = msg.err.Error()
		}

		// Сохраняем state после копирования (даже при частичной ошибке).
		if m.st != nil {
			records := make([]state.Record, 0, len(m.entries))
			for _, e := range m.entries {
				records = append(records, state.Record{
					SourcePath: e.Source.Path,
					Size:       e.Source.Size,
					ModTime:    e.Source.ModTime,
					FastHash:   m.fastHashes[e.Source.Path],
					FullHash:   m.fullHashes[e.Source.Path],
					TargetPath: e.Target,
				})
			}
			_ = m.st.Update(records)
			_ = m.st.Cleanup(m.allPaths)
			_ = m.st.Close()
			m.st = nil
		}

		m = m.logCopyResult()
		if m.GetSettingBool("notify") && notify.Available() {
			summary := notify.Summary{
				Total:   len(m.files),
				Copied:  m.copy.stats.Copied,
				Skipped: m.copy.stats.Skipped,
				Errors:  m.copy.stats.Errors,
			}
			_ = notify.Send(summary.Title(), summary.Body())
		}
		return m, nil

	case tea.KeyMsg:
		if m.copy.running {
			if msg.Type == tea.KeyEsc {
				if m.copyCancel != nil {
					m.copyCancel()
				}
				m.copy.running = false
				m.copy.aborted = true
				m.copy.errMsg = "Отменено пользователем"
				return m, nil
			}
			return m, nil
		}
		if m.copy.done || m.copy.errMsg != "" {
			switch msg.String() {
			case "enter":
				return m.resetToSources()
			case "esc":
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m Model) logCopyResult() Model {
	logPath := filepath.Join(m.Target, time.Now().Format("2006-01-02_15-04-05")+"_photo-sorter.log")
	l, err := logger.New(logPath)
	if err != nil {
		m.copy.logErr = fmt.Sprintf("Не удалось создать лог: %v", err)
		return m
	}
	defer l.Close()

	l.Log(fmt.Sprintf("Sources: %s", strings.Join(m.Sources, ", ")))
	l.Log(fmt.Sprintf("Target: %s", m.Target))
	l.Log(fmt.Sprintf("Files found: %d", len(m.files)))
	l.Log(fmt.Sprintf("Copied: %d", m.copy.stats.Copied))
	l.Log(fmt.Sprintf("Skipped (duplicates): %d", m.copy.stats.Skipped))
	l.Log(fmt.Sprintf("Errors: %d", m.copy.stats.Errors))
	if m.copy.stats.IntegrityFailures > 0 {
		l.Log(fmt.Sprintf("Integrity failures: %d", m.copy.stats.IntegrityFailures))
	}
	l.Log(fmt.Sprintf("Bytes copied: %d", m.copy.stats.BytesCopied))
	for _, e := range m.copy.stats.ErrorList {
		l.Log(fmt.Sprintf("Error detail: %s", e.Error()))
	}
	strategy := m.GetSettingString("dup_strategy")
	for _, dupGroup := range m.duplicates {
		for _, dup := range dupGroup.Duplicates {
			_ = l.LogDuplicate(dupGroup.Original.Path, dup.Path, strategy)
		}
	}
	if m.copy.errMsg != "" {
		l.Log(fmt.Sprintf("Fatal error: %s", m.copy.errMsg))
	}
	return m
}

func (m Model) viewCopy() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(" photo-sorter "))
	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render("Шаг 6. Копирование файлов"))
	b.WriteString("\n\n")

	if m.copy.errMsg != "" {
		b.WriteString(errorStyle.Render("Ошибка: "+m.copy.errMsg) + "\n\n")
		if m.copy.logErr != "" {
			b.WriteString(errorStyle.Render("⚠ "+m.copy.logErr) + "\n\n")
		}
		b.WriteString(helpStyle.Render("enter — начать заново • esc — выход"))
		return b.String()
	}

	barWidth := 30
	progress := 0.0
	if m.copy.total > 0 {
		progress = float64(m.copy.current) / float64(m.copy.total) * 100
	}
	if m.copy.done {
		progress = 100
	}
	filled := int(progress / 100.0 * float64(barWidth))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	b.WriteString(fmt.Sprintf("[%s] %.0f%%\n", bar, progress))
	b.WriteString("\n")

	if m.copy.running {
		b.WriteString(fmt.Sprintf("Обработано %d из %d…\n", m.copy.current, m.copy.total))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("esc — отмена"))
	} else if m.copy.done {
		b.WriteString(successStyle.Render("✓ Копирование завершено!"))
		b.WriteString("\n\n")
		if m.copy.logErr != "" {
			b.WriteString(errorStyle.Render("⚠ "+m.copy.logErr) + "\n\n")
		}
		b.WriteString(fmt.Sprintf("Скопировано: %d\n", m.copy.stats.Copied))
		b.WriteString(fmt.Sprintf("Пропущено (дубли): %d\n", m.copy.stats.Skipped))
		b.WriteString(fmt.Sprintf("Ошибок: %d\n", m.copy.stats.Errors))
		if m.copy.stats.ExifWrites > 0 {
			b.WriteString(fmt.Sprintf("EXIF записан: %d\n", m.copy.stats.ExifWrites))
		}
		if m.copy.stats.ExifFailures > 0 {
			b.WriteString(errorStyle.Render(fmt.Sprintf("Ошибок EXIF: %d", m.copy.stats.ExifFailures)) + "\n")
		}
		if m.copy.stats.IntegrityFailures > 0 {
			b.WriteString(errorStyle.Render(fmt.Sprintf("Ошибок целостности: %d", m.copy.stats.IntegrityFailures)) + "\n")
		}
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("enter — начать заново • esc — выход"))
	}

	return b.String()
}
