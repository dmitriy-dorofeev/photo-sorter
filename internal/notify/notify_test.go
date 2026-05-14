package notify

import (
	"testing"
)

func TestSummaryTitle(t *testing.T) {
	tests := []struct {
		name     string
		summary  Summary
		expected string
	}{
		{"no errors", Summary{Total: 10, Copied: 8, Skipped: 2, Errors: 0}, "Сортировка завершена"},
		{"with errors", Summary{Total: 10, Copied: 7, Skipped: 2, Errors: 1}, "Сортировка завершена с ошибками"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.summary.Title()
			if got != tt.expected {
				t.Errorf("Title() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSummaryBody(t *testing.T) {
	tests := []struct {
		name     string
		summary  Summary
		expected string
	}{
		{"copied only", Summary{Copied: 100}, "Скопировано: 100"},
		{"copied and skipped", Summary{Copied: 80, Skipped: 20}, "Скопировано: 80, Пропущено: 20"},
		{"all fields", Summary{Copied: 80, Skipped: 15, Errors: 5}, "Скопировано: 80, Пропущено: 15, Ошибок: 5"},
		{"copied and errors", Summary{Copied: 90, Errors: 10}, "Скопировано: 90, Ошибок: 10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.summary.Body()
			if got != tt.expected {
				t.Errorf("Body() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestAvailableOnCurrentPlatform(t *testing.T) {
	// Дымовой тест: Available не должен паниковать.
	_ = Available()
}

func TestSendUnsupported(t *testing.T) {
	// Дымовой тест: Send не должен паниковать на пустых строках.
	// На macOS/Linux это реально запустит osascript/notify-send, что допустимо в тесте.
	_ = Send("", "")
}
