// Package logger ведёт журнал операций копирования.
package logger

import (
	"fmt"
	"os"
	"time"
)

// Logger записывает статистику и ошибки в файл.
type Logger struct {
	file *os.File
}

// New создаёт новый Logger, записывающий в указанный файл.
func New(path string) (*Logger, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &Logger{file: f}, nil
}

// Close закрывает файл лога.
func (l *Logger) Close() error {
	return l.file.Close()
}

// Log записывает строку в лог.
func (l *Logger) Log(msg string) {
	timestamp := time.Now().Format(time.RFC3339)
	fmt.Fprintf(l.file, "[%s] %s\n", timestamp, msg)
}
