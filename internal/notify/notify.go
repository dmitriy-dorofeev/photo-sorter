// Package notify отправляет системные уведомления по завершении операции.
package notify

import (
	"fmt"
	"strings"
)

// Summary содержит краткую статистику для уведомления.
type Summary struct {
	Total   int
	Copied  int
	Skipped int
	Errors  int
}

// Title возвращает заголовок уведомления.
func (s Summary) Title() string {
	if s.Errors > 0 {
		return "Сортировка завершена с ошибками"
	}
	return "Сортировка завершена"
}

// Body возвращает текст уведомления.
func (s Summary) Body() string {
	parts := []string{
		fmt.Sprintf("Скопировано: %d", s.Copied),
	}
	if s.Skipped > 0 {
		parts = append(parts, fmt.Sprintf("Пропущено: %d", s.Skipped))
	}
	if s.Errors > 0 {
		parts = append(parts, fmt.Sprintf("Ошибок: %d", s.Errors))
	}
	return strings.Join(parts, ", ")
}
