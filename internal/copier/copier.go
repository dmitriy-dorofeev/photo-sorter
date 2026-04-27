// Package copier отвечает за безопасное копирование файлов.
package copier

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"photo-sorter/internal/hasher"
	"photo-sorter/internal/sorter"

	"golang.org/x/sys/unix"
)

// Stats содержит результат операции копирования.
type Stats struct {
	Copied      int
	Skipped     int
	Errors      int
	BytesCopied int64
	ErrorList   []error // до 10 первых ошибок для отчёта
}

// Copier выполняет копирование файлов.
type Copier struct {
	dryRun     bool
	targetRoot string
	spaceFunc  func(string) (uint64, error)
}

// New создаёт новый Copier.
// targetRoot используется для проверки свободного места.
func New(dryRun bool, targetRoot string) *Copier {
	return &Copier{dryRun: dryRun, targetRoot: targetRoot, spaceFunc: availableSpace}
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
	syncDirs := make(map[string]struct{})

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

		// Защита от zero-value Copier: без targetRoot не можем валидировать пути.
		if c.targetRoot == "" {
			stats.Errors++
			c.recordError(&stats, fmt.Errorf("target root is empty"))
			continue
		}

		if err := validateTargetPath(c.targetRoot, e.Target); err != nil {
			stats.Errors++
			c.recordError(&stats, err)
			if c.shouldAbort(&consecutiveErrors, err) {
				return stats, fmt.Errorf("too many consecutive target errors (%d), aborting", consecutiveErrors)
			}
			continue
		}

		if c.dryRun {
			stats.Copied++
			consecutiveErrors = 0
			continue
		}

		dir := filepath.Dir(e.Target)
		if err := os.MkdirAll(dir, 0750); err != nil {
			stats.Errors++
			c.recordError(&stats, err)
			if c.shouldAbort(&consecutiveErrors, err) {
				return stats, fmt.Errorf("too many consecutive target errors (%d), aborting", consecutiveErrors)
			}
			continue
		}
		syncDirs[dir] = struct{}{}

		target := e.Target
		if info, err := os.Lstat(target); err == nil {
			// Целевой файл уже существует.
			if info.Mode()&os.ModeSymlink != 0 {
				// Symlink — удаляем перед копированием, чтобы не писать
				// через него и не оставлять потенциальную уязвимость.
				if err := os.Remove(target); err != nil {
					stats.Errors++
					c.recordError(&stats, err)
					continue
				}
			} else {
				hSrc, err1 := hasher.HashFile(ctx, e.Source.Path)
				hDst, err2 := hasher.HashFile(ctx, target)
				if err1 == nil && err2 == nil && hSrc == hDst {
					stats.Skipped++
					consecutiveErrors = 0
					continue
				}
				newTarget, err := findFreeName(target)
				if err != nil {
					stats.Errors++
					c.recordError(&stats, err)
					if c.shouldAbort(&consecutiveErrors, err) {
						return stats, fmt.Errorf("too many consecutive target errors (%d), aborting", consecutiveErrors)
					}
					continue
				}
				target = newTarget
			}
		}

		if err := copyFile(ctx, e.Source.Path, target); err != nil {
			if errors.Is(err, errSkipCollision) {
				stats.Skipped++
				consecutiveErrors = 0
				continue
			}
			stats.Errors++
			c.recordError(&stats, err)
			if c.shouldAbort(&consecutiveErrors, err) {
				return stats, fmt.Errorf("too many consecutive target errors (%d), aborting", consecutiveErrors)
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

	// Синхронизируем все затронутые поддиректории и корень,
	// чтобы метаданные файловой системы гарантированно записались на диск.
	if !c.dryRun && stats.Copied > 0 && c.targetRoot != "" {
		syncDirs[c.targetRoot] = struct{}{}
		for d := range syncDirs {
			if err := syncDir(d); err != nil {
				// ignore fsync errors on directories
			}
		}
	}

	return stats, nil
}

// syncDir вызывает fsync на директории, гарантируя сброс
// метаданных файловой системы на диск.
func syncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

// recordError добавляет ошибку в ErrorList (максимум 10 записей).
func (c *Copier) recordError(stats *Stats, err error) {
	if len(stats.ErrorList) < 10 {
		stats.ErrorList = append(stats.ErrorList, err)
	}
}

// isTargetError возвращает true, если ошибка связана с целевым диском/директорией.
// Ошибки вида "файл не найден" (исходный файл удалился между scan и copy)
// считаются source-ошибками и не приводят к abort.
func isTargetError(err error) bool {
	if err == nil {
		return false
	}
	// Файл не найден — скорее всего исходный файл удалился, это не target error.
	if os.IsNotExist(err) {
		return false
	}
	// Все остальные ошибки (permission denied, disk full, not a directory и т.д.)
	// считаются target-related.
	return true
}

// shouldAbort проверяет, не отключился ли целевой диск.
// Возвращает true, если 3+ target-ошибок подряд и targetRoot недоступен
// (не существует, нет прав, или не является директорией).
func (c *Copier) shouldAbort(consecutiveErrors *int, err error) bool {
	*consecutiveErrors++
	if *consecutiveErrors < 3 {
		return false
	}
	if !isTargetError(err) {
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
	if c.dryRun {
		return nil
	}
	var needed int64
	for _, e := range entries {
		if !e.Skip {
			needed += e.Source.Size
		}
	}
	if needed == 0 {
		return nil
	}
	available, err := c.spaceFunc(c.targetRoot)
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

// validateTargetPath проверяет, что target находится внутри targetRoot.
// Защита от path traversal через имя файла (например, "../../../etc/passwd").
func validateTargetPath(targetRoot, target string) error {
	absRoot, err := filepath.Abs(targetRoot)
	if err != nil {
		return err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return err
	}
	if strings.HasPrefix(rel, "..") {
		return fmt.Errorf("path traversal detected: %s", target)
	}
	return nil
}

func findFreeName(target string) (string, error) {
	dir := filepath.Dir(target)
	ext := filepath.Ext(target)
	base := strings.TrimSuffix(filepath.Base(target), ext)
	const maxIterations = 10000
	for i := 1; i <= maxIterations; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("cannot find free name for %s after %d attempts", target, maxIterations)
}

// errSkipCollision возвращается copyFile, если целевой файл неожиданно
// появился с тем же содержимым (TOCTOU-защита после findFreeName).
var errSkipCollision = errors.New("target collision with identical content")

func copyFile(ctx context.Context, src, dst string) error {
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
		if err := tmpFile.Close(); err != nil {
			// ignore cleanup close error
		}
		if cleanup {
			if err := os.Remove(tmpPath); err != nil {
				// ignore cleanup remove error
			}
		}
	}()

	if _, err := io.Copy(tmpFile, sourceFile); err != nil {
		return err
	}

	// Sync() убран отсюда по соображениям производительности.
	// Один sync на targetRoot выполняется в конце Copy.
	if err := tmpFile.Close(); err != nil {
		return err
	}

	// Финальная TOCTOU-защита: проверяем dst непосредственно перед rename.
	// Если dst — symlink, удаляем (не пишем через него).
	// Если dst — обычный файл, появившийся после findFreeName, сравниваем хеши:
	//   совпадают — пропускаем, разные — ошибка (не перезаписываем неожиданные данные).
	if info, err := os.Lstat(dst); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(dst); err != nil {
				return err
			}
		} else if info.Mode().IsRegular() {
			hSrc, err1 := hasher.HashFile(ctx, src)
			hDst, err2 := hasher.HashFile(ctx, dst)
			if err1 == nil && err2 == nil && hSrc == hDst {
				// Тот же файл — пропускаем, удаляем temp.
				cleanup = false
				if err := os.Remove(tmpPath); err != nil {
					// ignore cleanup remove error
				}
				return errSkipCollision
			}
			return fmt.Errorf("target collision detected: %s appeared unexpectedly", dst)
		}
	}

	if err := os.Rename(tmpPath, dst); err != nil {
		return err
	}
	cleanup = false
	return nil
}
