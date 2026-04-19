package tui

import "strings"

func (m Model) viewPreview() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" photo-sorter "))
	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render("Шаг 5. Предпросмотр (dry-run)"))
	b.WriteString("\n\n")
	b.WriteString("Здесь будет дерево целевой структуры, список дублей и unknown_date.\n")
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("esc — назад • enter — запустить копирование"))
	return b.String()
}

func (m Model) viewCopy() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" photo-sorter "))
	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render("Шаг 6. Копирование файлов"))
	b.WriteString("\n\n")
	b.WriteString("[                    ] 0%\n")
	b.WriteString("Скопировано: 0\n")
	b.WriteString("Пропущено (дубли): 0\n")
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("esc — выход по завершении"))
	return b.String()
}
