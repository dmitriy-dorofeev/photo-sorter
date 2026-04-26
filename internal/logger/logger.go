// Package logger ведёт журнал операций копирования.
package logger

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Logger записывает статистику и ошибки в файл.
type Logger struct {
	file *os.File
	mu   sync.Mutex
}

// New создаёт новый Logger, записывающий в указанный файл.
func New(path string) (*Logger, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &Logger{file: f}, nil
}

// Close синхронизирует и закрывает файл лога.
func (l *Logger) Close() error {
	_ = l.file.Sync()
	return l.file.Close()
}

// Log записывает строку в лог.
func (l *Logger) Log(msg string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	timestamp := time.Now().Format(time.RFC3339)
	_, err := fmt.Fprintf(l.file, "[%s] %s\n", timestamp, msg)
	return err
}
