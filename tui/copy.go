package tui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"photo-sorter/internal/copier"
	"photo-sorter/internal/logger"
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

func (m Model) startCopy() tea.Cmd {
	// Отменяем предыдущее копирование, если оно ещё бежит.
	if m.copyCancel != nil {
		m.copyCancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.copyCancel = cancel

	dryRun := false

	return func() tea.Msg {
		c := copier.New(dryRun, m.Target)
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
		if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
			m.copy.errMsg = msg.err.Error()
		} else if msg.err == nil {
			m.copy.stats = msg.stats
		}
		m.logCopyResult()
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

func (m Model) logCopyResult() {
	logPath := filepath.Join(m.Target, time.Now().Format("2006-01-02_15-04-05")+"_photo-sorter.log")
	l, err := logger.New(logPath)
	if err != nil {
		m.copy.logErr = fmt.Sprintf("Не удалось создать лог: %v", err)
		return
	}
	defer l.Close()

	l.Log(fmt.Sprintf("Sources: %s", strings.Join(m.Sources, ", ")))
	l.Log(fmt.Sprintf("Target: %s", m.Target))
	l.Log(fmt.Sprintf("Files found: %d", len(m.files)))
	l.Log(fmt.Sprintf("Copied: %d", m.copy.stats.Copied))
	l.Log(fmt.Sprintf("Skipped (duplicates): %d", m.copy.stats.Skipped))
	l.Log(fmt.Sprintf("Errors: %d", m.copy.stats.Errors))
	l.Log(fmt.Sprintf("Bytes copied: %d", m.copy.stats.BytesCopied))
	for _, e := range m.copy.stats.ErrorList {
		l.Log(fmt.Sprintf("Error detail: %s", e.Error()))
	}
	if m.copy.errMsg != "" {
		l.Log(fmt.Sprintf("Fatal error: %s", m.copy.errMsg))
	}
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
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("enter — начать заново • esc — выход"))
	} else {
		b.WriteString(helpStyle.Render("enter — начать копирование • esc — выход"))
	}

	return b.String()
}
