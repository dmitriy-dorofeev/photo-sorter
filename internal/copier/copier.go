// Package copier отвечает за безопасное копирование файлов.
package copier

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
	"photo-sorter/internal/deduper"
	"photo-sorter/internal/sorter"
)

// Stats содержит результат операции копирования.
type Stats struct {
	Copied      int
	Skipped     int
	Errors      int
	BytesCopied int64
	ErrorList   []string // до 10 первых ошибок для отчёта
}

// Copier выполняет копирование файлов.
type Copier struct {
	dryRun     bool
	targetRoot string
}

// New создаёт новый Copier.
// targetRoot используется для проверки свободного места.
func New(dryRun bool, targetRoot string) *Copier {
	return &Copier{dryRun: dryRun, targetRoot: targetRoot}
}

// Copy выполняет копирование по плану сортировки.
// progress вызывается после обработки каждого элемента (current из [0,total]).
func (c *Copier) Copy(
	ctx context.Context,
	entries []sorter.Entry,
	progress func(current, total int),
) (Stats, error) {
	var stats Stats
	total := len(entries)
	consecutiveErrors := 0

	if !c.dryRun && c.targetRoot != "" && total > 0 {
		if err := c.checkDiskSpace(entries); err != nil {
			return stats, err
		}
	}

	for i, e := range entries {
		if progress != nil {
			progress(i, total)
		}

		select {
		case <-ctx.Done():
			if progress != nil {
				progress(total, total)
			}
			return stats, ctx.Err()
		default:
		}

		if e.Skip {
			stats.Skipped++
			consecutiveErrors = 0
			continue
		}

		if c.dryRun {
			stats.Copied++
			consecutiveErrors = 0
			continue
		}

		if err := os.MkdirAll(filepath.Dir(e.Target), 0755); err != nil {
			stats.Errors++
			c.recordError(&stats, err)
			if c.shouldAbort(&consecutiveErrors) {
				return stats, fmt.Errorf("target disk unavailable after %d consecutive errors", consecutiveErrors)
			}
			continue
		}

		target := e.Target
		if _, err := os.Stat(target); err == nil {
			// Целевой файл уже существует.
			hSrc, err1 := deduper.HashFile(e.Source.Path)
			hDst, err2 := deduper.HashFile(target)
			if err1 == nil && err2 == nil && hSrc == hDst {
				stats.Skipped++
				consecutiveErrors = 0
				continue
			}
			target = findFreeName(target)
		}

		if err := copyFile(e.Source.Path, target); err != nil {
			stats.Errors++
			c.recordError(&stats, err)
			if c.shouldAbort(&consecutiveErrors) {
				return stats, fmt.Errorf("target disk unavailable after %d consecutive errors", consecutiveErrors)
			}
			continue
		}
		stats.Copied++
		stats.BytesCopied += e.Source.Size
		consecutiveErrors = 0
	}

	if progress != nil {
		progress(total, total)
	}
	return stats, nil
}

// recordError добавляет ошибку в ErrorList (максимум 10 записей).
func (c *Copier) recordError(stats *Stats, err error) {
	if len(stats.ErrorList) < 10 {
		stats.ErrorList = append(stats.ErrorList, err.Error())
	}
}

// shouldAbort проверяет, не отключился ли целевой диск.
// Возвращает true, если 3+ ошибок подряд и targetRoot недоступен
// (не существует, нет прав, или не является директорией).
func (c *Copier) shouldAbort(consecutiveErrors *int) bool {
	*consecutiveErrors++
	if *consecutiveErrors < 3 {
		return false
	}
	if c.targetRoot == "" {
		return false
	}
	info, err := os.Stat(c.targetRoot)
	if os.IsNotExist(err) || os.IsPermission(err) {
		return true
	}
	if err == nil && !info.IsDir() {
		return true
	}
	return false
}

func (c *Copier) checkDiskSpace(entries []sorter.Entry) error {
	var needed int64
	for _, e := range entries {
		if !e.Skip {
			needed += e.Source.Size
		}
	}
	if needed == 0 {
		return nil
	}
	available, err := availableSpace(c.targetRoot)
	if err != nil {
		return fmt.Errorf("cannot check disk space: %w", err)
	}
	if available < uint64(needed) {
		return fmt.Errorf("not enough disk space: need %d bytes, have %d bytes", needed, available)
	}
	return nil
}

func availableSpace(path string) (uint64, error) {
	var stat unix.Statfs_t
	err := unix.Statfs(path, &stat)
	if err != nil {
		// Если сам путь не существует, пробуем родительскую директорию.
		if os.IsNotExist(err) {
			parent := filepath.Dir(path)
			if parent != path {
				err = unix.Statfs(parent, &stat)
			}
		}
		if err != nil {
			return 0, err
		}
	}
	return stat.Bavail * uint64(stat.Bsize), nil
}

func findFreeName(target string) string {
	dir := filepath.Dir(target)
	ext := filepath.Ext(target)
	base := strings.TrimSuffix(filepath.Base(target), ext)
	for i := 1; ; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Создаём временный файл в той же директории для атомарного rename.
	// Это решает две проблемы:
	// 1. При ошибке io.Copy не остаётся битого файла на месте назначения.
	// 2. Не следуем по symlink (O_EXCL + rename заменяет symlink, а не пишет через него).
	dir := filepath.Dir(dst)
	tmpFile, err := os.CreateTemp(dir, filepath.Base(dst)+".tmp.")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	// cleanup удаляет временный файл при любой ошибке.
	cleanup := true
	defer func() {
		tmpFile.Close()
		if cleanup {
			os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(tmpFile, sourceFile); err != nil {
		return err
	}

	if err := tmpFile.Sync(); err != nil {
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, dst); err != nil {
		return err
	}
	cleanup = false
	return nil
}
