// Package logger ведёт журнал операций копирования.
package logger

import (
	"errors"
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
	// #nosec G304 — путь формируется внутри приложения (целевая директория + фиксированное имя).
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &Logger{file: f}, nil
}

// Close синхронизирует и закрывает файл лога.
func (l *Logger) Close() error {
	syncErr := l.file.Sync()
	closeErr := l.file.Close()
	return errors.Join(syncErr, closeErr)
}

// Log записывает строку в лог.
func (l *Logger) Log(msg string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	timestamp := time.Now().Format(time.RFC3339)
	_, err := fmt.Fprintf(l.file, "[%s] %s\n", timestamp, msg)
	return err
}

// LogDuplicate записывает информацию о пропущенном дубликате.
func (l *Logger) LogDuplicate(original, duplicate string, strategy string) error {
	msg := fmt.Sprintf("DUPLICATE: kept %s (strategy=%s), skipped %s (same hash)", original, strategy, duplicate)
	return l.Log(msg)
}

// LogIntegrityFailure записывает информацию о несовпадении хешей после копирования.
func (l *Logger) LogIntegrityFailure(src, dst string, srcHash, dstHash uint64) error {
	msg := fmt.Sprintf("INTEGRITY FAILURE: %s -> %s (src_hash=%x dst_hash=%x)", src, dst, srcHash, dstHash)
	return l.Log(msg)
}
