// Package copier отвечает за безопасное копирование файлов.
package copier

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"photo-sorter/internal/deduper"
	"photo-sorter/internal/sorter"
	"golang.org/x/sys/unix"
)

// Stats содержит результат операции копирования.
type Stats struct {
	Copied      int
	Skipped     int
	Errors      int
	BytesCopied int64
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
			continue
		}

		if c.dryRun {
			stats.Copied++
			continue
		}

		if err := os.MkdirAll(filepath.Dir(e.Target), 0755); err != nil {
			stats.Errors++
			continue
		}

		target := e.Target
		if _, err := os.Stat(target); err == nil {
			// Целевой файл уже существует.
			hSrc, err1 := deduper.HashFile(e.Source.Path)
			hDst, err2 := deduper.HashFile(target)
			if err1 == nil && err2 == nil && hSrc == hDst {
				stats.Skipped++
				continue
			}
			target = findFreeName(target)
		}

		if err := copyFile(e.Source.Path, target); err != nil {
			stats.Errors++
			continue
		}
		stats.Copied++
		stats.BytesCopied += e.Source.Size
	}

	if progress != nil {
		progress(total, total)
	}
	return stats, nil
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
		return 0, err
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

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	return destFile.Sync()
}
