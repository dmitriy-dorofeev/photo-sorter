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
	"photo-sorter/internal/notify"
	"photo-sorter/internal/report"
	"photo-sorter/internal/sorter"
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
	width     int
	height    int
	running   bool
	current   int
	total     int
	done      bool
	errMsg    string
	stats     copier.Stats
	aborted   bool   // true если пользователь нажал Esc
	logErr    string // ошибка создания отчёта (предупреждение)
	reportMsg string // путь к сохранённому отчёту (успех)
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
		c.Concurrency = m.concurrency()
		c.WriteExif = m.GetSettingBool("write_exif") && m.exifToolPath != ""
		c.WriteSpotlight = m.GetSettingBool("write_spotlight")
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
	var dupGroups []report.DupGroup
	strategy := m.GetSettingString("dup_strategy")
	for _, g := range m.duplicates {
		dups := make([]string, len(g.Duplicates))
		for i, d := range g.Duplicates {
			dups[i] = d.Path
		}
		dupGroups = append(dupGroups, report.DupGroup{
			Original:   g.Original.Path,
			Duplicates: dups,
			Strategy:   strategy,
		})
	}

	var unsortedFiles []string
	for _, e := range m.entries {
		if !e.Skip && sorter.IsUnsorted(e.Target) {
			unsortedFiles = append(unsortedFiles, e.Source.Path)
		}
	}

	data := report.Data{
		Sources:           m.Sources,
		Target:            m.Target,
		FilesFound:        len(m.files),
		Copied:            m.copy.stats.Copied,
		Skipped:           m.copy.stats.Skipped,
		Errors:            m.copy.stats.Errors,
		IntegrityFailures: m.copy.stats.IntegrityFailures,
		ExifWrites:        m.copy.stats.ExifWrites,
		ExifFailures:      m.copy.stats.ExifFailures,
		BytesCopied:       m.copy.stats.BytesCopied,
		ErrorList:         m.copy.stats.ErrorList,
		Duplicates:        dupGroups,
		UnsortedFiles:     unsortedFiles,
		FatalError:        m.copy.errMsg,
	}

	reportFormat := m.GetSettingString("report_format")
	path, err := report.Generate(m.Target, reportFormat, data)
	if err != nil {
		m.copy.logErr = fmt.Sprintf("Не удалось сохранить отчёт: %v", err)
		return m
	}
	m.copy.reportMsg = fmt.Sprintf("Отчёт сохранён: %s", filepath.Base(path))
	return m
}

func (m Model) viewCopy() string {
	var b strings.Builder

	b.WriteString(m.theme.Title.Render(" photo-sorter "))
	b.WriteString("\n\n")
	b.WriteString(m.theme.Subtitle.Render("Шаг 6. Копирование файлов"))
	b.WriteString("\n\n")

	if m.copy.errMsg != "" {
		b.WriteString(m.theme.Error.Render("Ошибка: "+m.copy.errMsg) + "\n\n")
		if m.copy.logErr != "" {
			b.WriteString(m.theme.Error.Render("⚠ "+m.copy.logErr) + "\n\n")
		}
		b.WriteString(m.theme.Help.Render("enter — начать заново • esc — выход"))
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
		b.WriteString(m.theme.Help.Render("esc — отмена"))
	} else if m.copy.done {
		b.WriteString(m.theme.Success.Render("✓ Копирование завершено!"))
		b.WriteString("\n\n")
		if m.copy.reportMsg != "" {
			b.WriteString(m.theme.Success.Render("📝 "+m.copy.reportMsg) + "\n\n")
		}
		if m.copy.logErr != "" {
			b.WriteString(m.theme.Error.Render("⚠ "+m.copy.logErr) + "\n\n")
		}
		b.WriteString(fmt.Sprintf("Скопировано: %d\n", m.copy.stats.Copied))
		b.WriteString(fmt.Sprintf("Пропущено (дубли): %d\n", m.copy.stats.Skipped))
		b.WriteString(fmt.Sprintf("Ошибок: %d\n", m.copy.stats.Errors))
		if m.copy.stats.ExifWrites > 0 {
			b.WriteString(fmt.Sprintf("EXIF записан: %d\n", m.copy.stats.ExifWrites))
		}
		if m.copy.stats.ExifFailures > 0 {
			b.WriteString(m.theme.Error.Render(fmt.Sprintf("Ошибок EXIF: %d", m.copy.stats.ExifFailures)) + "\n")
		}
		if m.copy.stats.SpotlightWrites > 0 {
			b.WriteString(fmt.Sprintf("Spotlight тегов: %d\n", m.copy.stats.SpotlightWrites))
		}
		if m.copy.stats.SpotlightFailures > 0 {
			b.WriteString(m.theme.Error.Render(fmt.Sprintf("Ошибок Spotlight: %d", m.copy.stats.SpotlightFailures)) + "\n")
		}
		if m.copy.stats.IntegrityFailures > 0 {
			b.WriteString(m.theme.Error.Render(fmt.Sprintf("Ошибок целостности: %d", m.copy.stats.IntegrityFailures)) + "\n")
		}
		b.WriteString("\n")
		b.WriteString(m.theme.Help.Render("enter — начать заново • esc — выход"))
	}

	return b.String()
}
