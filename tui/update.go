package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"photo-sorter/internal/updater"
)

// updateCheckMsg передаёт результат асинхронной проверки обновления.
type updateCheckMsg struct {
	result updater.CheckResult
}

// checkUpdateCmd запускает проверку обновления в фоне.
func checkUpdateCmd(version string) tea.Cmd {
	return func() tea.Msg {
		res := updater.CheckVersion(version)
		return updateCheckMsg{result: res}
	}
}

// updateNotice возвращает строку с уведомлением о версии/обновлении для отображения в TUI.
func updateNotice(m Model) string {
	if m.updateResult == nil {
		return ""
	}

	res := *m.updateResult

	if res.IsDirty {
		return errorStyle.Render("⚠ Версия собрана из 'грязного' дерева — обновление невозможно")
	}

	if res.IsDev {
		return subtitleStyle.Render("Версия dev — обновления недоступны")
	}

	if res.Error != nil {
		return ""
	}

	if res.HasUpdate {
		return successStyle.Render(fmt.Sprintf("⬆ Доступно обновление: %s → %s", m.version, res.Latest))
	}

	return ""
}
