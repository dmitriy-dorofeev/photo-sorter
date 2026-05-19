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
	"sync"
	"sync/atomic"

	"photo-sorter/internal/collision"
	"photo-sorter/internal/dateresolver"
	"photo-sorter/internal/hasher"
	"photo-sorter/internal/sorter"
	"photo-sorter/internal/spotlight"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/unix"
)

// Stats содержит результат операции копирования.
type Stats struct {
	Copied            int
	Skipped           int
	Errors            int
	IntegrityFailures int
	ExifWrites        int
	ExifFailures      int
	SpotlightWrites   int
	SpotlightFailures int
	BytesCopied       int64
	ErrorList         []error // до 10 первых ошибок для отчёта
}

// Copier выполняет копирование файлов.
type Copier struct {
	dryRun            bool
	targetRoot        string
	spaceFunc         func(string) (uint64, error)
	hashFunc          func(context.Context, string) (uint64, error)
	collisionStrategy collision.Strategy
	WriteExif         bool
	WriteSpotlight    bool
	ExifToolPath      string
	Concurrency       int      // число потоков; ≤1 — последовательный режим
	dirLocks          sync.Map // string → *sync.Mutex
}

// New создаёт новый Copier.
// targetRoot используется для проверки свободного места.
// collisionStrategy может быть пустой — тогда используется counter.
func New(dryRun bool, targetRoot string, collisionStrategy collision.Strategy) *Copier {
	if collisionStrategy == "" {
		collisionStrategy = collision.StrategyCounter
	}
	return &Copier{dryRun: dryRun, targetRoot: targetRoot, spaceFunc: availableSpace, hashFunc: hasher.HashFile, collisionStrategy: collisionStrategy}
}

// Copy выполняет копирование по плану сортировки.
// progress вызывается после обработки каждого элемента (current из [0,total]).
func (c *Copier) Copy(
	ctx context.Context,
	entries []sorter.Entry,
	progress func(current, total int),
) (Stats, error) {
	if c.Concurrency <= 1 {
		return c.copySequential(ctx, entries, progress)
	}
	return c.copyParallel(ctx, entries, progress)
}

// copySequential выполняет последовательное копирование.
func (c *Copier) copySequential(
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

	for i := range entries {
		e := &entries[i]
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
				hSrc, err1 := c.hashFunc(ctx, e.Source.Path)
				hDst, err2 := c.hashFunc(ctx, target)
				if err1 == nil && err2 == nil && hSrc == hDst {
					stats.Skipped++
					consecutiveErrors = 0
					continue
				}
				newTarget, err := c.resolveCollision(target, e.Source.Path)
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

		e.Target = target

		if err := c.copyFile(ctx, e.Source.Path, target); err != nil {
			if errors.Is(err, errSkipCollision) {
				stats.Skipped++
				consecutiveErrors = 0
				continue
			}
			if errors.Is(err, errIntegrityCheck) {
				stats.IntegrityFailures++
			}
			stats.Errors++
			c.recordError(&stats, err)
			if c.shouldAbort(&consecutiveErrors, err) {
				return stats, fmt.Errorf("too many consecutive target errors (%d), aborting", consecutiveErrors)
			}
			continue
		}

		// Обратная синхронизация: если дата взята из имени или mtime, записываем её в EXIF.
		if c.WriteExif && !e.Skip && isWritableImage(e.Source.Ext) &&
			(e.DateSource == dateresolver.SourceFilename || e.DateSource == dateresolver.SourceModTime) {
			if err := writeExifDate(ctx, c.ExifToolPath, target, e.Date); err != nil {
				stats.ExifFailures++
				c.recordError(&stats, fmt.Errorf("exif write failed for %s: %w", target, err))
			} else {
				stats.ExifWrites++
			}
		}

		if c.WriteSpotlight && spotlight.Available() && !e.Skip && !e.Date.IsZero() {
			if err := spotlight.WriteTags(target, e.Date); err != nil {
				stats.SpotlightFailures++
				c.recordError(&stats, fmt.Errorf("spotlight write failed for %s: %w", target, err))
			} else {
				stats.SpotlightWrites++
			}
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

// copyParallel выполняет параллельное копирование через errgroup.
func (c *Copier) copyParallel(
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

	var (
		statsMu    sync.Mutex
		entryMu    sync.Mutex
		progressMu sync.Mutex
		syncDirsMu sync.Mutex
		syncDirs   = make(map[string]struct{})
		processed  atomic.Int64
		targetErrs atomic.Int32
	)

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(c.Concurrency)

	for i := range entries {
		i := i
		e := &entries[i]

		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if e.Skip {
				statsMu.Lock()
				stats.Skipped++
				statsMu.Unlock()
				c.reportProgress(progress, &progressMu, &processed, total)
				return nil
			}

			if c.targetRoot == "" {
				statsMu.Lock()
				stats.Errors++
				c.recordError(&stats, fmt.Errorf("target root is empty"))
				statsMu.Unlock()
				c.reportProgress(progress, &progressMu, &processed, total)
				return nil
			}

			if err := validateTargetPath(c.targetRoot, e.Target); err != nil {
				statsMu.Lock()
				stats.Errors++
				c.recordError(&stats, err)
				abort := c.shouldAbortParallel(err, &targetErrs)
				statsMu.Unlock()
				c.reportProgress(progress, &progressMu, &processed, total)
				if abort {
					return fmt.Errorf("too many target errors, aborting")
				}
				return nil
			}

			if c.dryRun {
				statsMu.Lock()
				stats.Copied++
				statsMu.Unlock()
				c.reportProgress(progress, &progressMu, &processed, total)
				return nil
			}

			dir := filepath.Dir(e.Target)
			actual, _ := c.dirLocks.LoadOrStore(dir, &sync.Mutex{})
			dirMu := actual.(*sync.Mutex)
			dirMu.Lock()

			if err := os.MkdirAll(dir, 0750); err != nil {
				dirMu.Unlock()
				statsMu.Lock()
				stats.Errors++
				c.recordError(&stats, err)
				abort := c.shouldAbortParallel(err, &targetErrs)
				statsMu.Unlock()
				c.reportProgress(progress, &progressMu, &processed, total)
				if abort {
					return fmt.Errorf("too many target errors, aborting")
				}
				return nil
			}
			syncDirsMu.Lock()
			syncDirs[dir] = struct{}{}
			syncDirsMu.Unlock()

			target := e.Target
			if info, err := os.Lstat(target); err == nil {
				if info.Mode()&os.ModeSymlink != 0 {
					if err := os.Remove(target); err != nil {
						dirMu.Unlock()
						statsMu.Lock()
						stats.Errors++
						c.recordError(&stats, err)
						statsMu.Unlock()
						c.reportProgress(progress, &progressMu, &processed, total)
						return nil
					}
				} else {
					hSrc, err1 := c.hashFunc(ctx, e.Source.Path)
					hDst, err2 := c.hashFunc(ctx, target)
					if err1 == nil && err2 == nil && hSrc == hDst {
						dirMu.Unlock()
						statsMu.Lock()
						stats.Skipped++
						statsMu.Unlock()
						c.reportProgress(progress, &progressMu, &processed, total)
						return nil
					}
					newTarget, err := c.resolveCollision(target, e.Source.Path)
					if err != nil {
						dirMu.Unlock()
						statsMu.Lock()
						stats.Errors++
						c.recordError(&stats, err)
						abort := c.shouldAbortParallel(err, &targetErrs)
						statsMu.Unlock()
						c.reportProgress(progress, &progressMu, &processed, total)
						if abort {
							return fmt.Errorf("too many target errors, aborting")
						}
						return nil
					}
					target = newTarget
				}
			}

			entryMu.Lock()
			entries[i].Target = target
			entryMu.Unlock()

			if err := c.copyFile(ctx, e.Source.Path, target); err != nil {
				dirMu.Unlock()
				if errors.Is(err, errSkipCollision) {
					statsMu.Lock()
					stats.Skipped++
					statsMu.Unlock()
					c.reportProgress(progress, &progressMu, &processed, total)
					return nil
				}
				statsMu.Lock()
				if errors.Is(err, errIntegrityCheck) {
					stats.IntegrityFailures++
				}
				stats.Errors++
				c.recordError(&stats, err)
				abort := c.shouldAbortParallel(err, &targetErrs)
				statsMu.Unlock()
				c.reportProgress(progress, &progressMu, &processed, total)
				if abort {
					return fmt.Errorf("too many target errors, aborting")
				}
				return nil
			}

			// Обратная синхронизация: если дата взята из имени или mtime, записываем её в EXIF.
			if c.WriteExif && !e.Skip && isWritableImage(e.Source.Ext) &&
				(e.DateSource == dateresolver.SourceFilename || e.DateSource == dateresolver.SourceModTime) {
				if err := writeExifDate(ctx, c.ExifToolPath, target, e.Date); err != nil {
					statsMu.Lock()
					stats.ExifFailures++
					c.recordError(&stats, fmt.Errorf("exif write failed for %s: %w", target, err))
					statsMu.Unlock()
				} else {
					statsMu.Lock()
					stats.ExifWrites++
					statsMu.Unlock()
				}
			}

			if c.WriteSpotlight && spotlight.Available() && !e.Skip && !e.Date.IsZero() {
				if err := spotlight.WriteTags(target, e.Date); err != nil {
					statsMu.Lock()
					stats.SpotlightFailures++
					c.recordError(&stats, fmt.Errorf("spotlight write failed for %s: %w", target, err))
					statsMu.Unlock()
				} else {
					statsMu.Lock()
					stats.SpotlightWrites++
					statsMu.Unlock()
				}
			}

			statsMu.Lock()
			stats.Copied++
			stats.BytesCopied += e.Source.Size
			statsMu.Unlock()

			dirMu.Unlock()
			c.reportProgress(progress, &progressMu, &processed, total)
			return nil
		})
	}

	err := g.Wait()

	// Синхронизируем все затронутые поддиректории и корень.
	if !c.dryRun && stats.Copied > 0 && c.targetRoot != "" {
		syncDirsMu.Lock()
		syncDirs[c.targetRoot] = struct{}{}
		dirs := make([]string, 0, len(syncDirs))
		for d := range syncDirs {
			dirs = append(dirs, d)
		}
		syncDirsMu.Unlock()
		for _, d := range dirs {
			if err := syncDir(d); err != nil {
				// ignore fsync errors on directories
			}
		}
	}

	return stats, err
}

// reportProgress атомарно инкрементирует счётчик и вызывает callback.
func (c *Copier) reportProgress(progress func(current, total int), mu *sync.Mutex, processed *atomic.Int64, total int) {
	if progress == nil {
		return
	}
	cur := int(processed.Add(1))
	mu.Lock()
	progress(cur, total)
	mu.Unlock()
}

// syncDir вызывает fsync на директории, гарантируя сброс
// метаданных файловой системы на диск.
func syncDir(path string) error {
	// #nosec G304 — путь — целевая директория, сформированная внутри приложения.
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

// shouldAbortParallel — версия shouldAbort для параллельного режима.
// Считает суммарное число target-ошибок (не обязательно подряд).
func (c *Copier) shouldAbortParallel(err error, targetErrors *atomic.Int32) bool {
	if !isTargetError(err) {
		return false
	}
	n := targetErrors.Add(1)
	if n < 3 {
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
	var needed uint64
	for _, e := range entries {
		if !e.Skip {
			if e.Source.Size >= 0 {
				// #nosec G115 — Size возвращается os.FileInfo и не может быть отрицательным для обычных файлов.
				needed += uint64(e.Source.Size)
			}
		}
	}
	if needed == 0 {
		return nil
	}
	available, err := c.spaceFunc(c.targetRoot)
	if err != nil {
		return fmt.Errorf("cannot check disk space: %w", err)
	}
	if available < needed {
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
	// #nosec G115 — Bsize (размер блока файловой системы) не может быть отрицательным.
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

func (c *Copier) resolveCollision(target, sourcePath string) (string, error) {
	const maxIterations = 10000
	for i := 0; i <= maxIterations; i++ {
		candidate := collision.Resolve(target, c.collisionStrategy, sourcePath, i)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("cannot find free name for %s after %d attempts", target, maxIterations)
}

// errSkipCollision возвращается copyFile, если целевой файл неожиданно
// появился с тем же содержимым (TOCTOU-защита после findFreeName).
var errSkipCollision = errors.New("target collision with identical content")

// errIntegrityCheck возвращается, если хеши исходника и копии не совпадают после записи.
var errIntegrityCheck = errors.New("integrity check failed")

func (c *Copier) copyFile(ctx context.Context, src, dst string) error {
	hashFunc := c.hashFunc
	if hashFunc == nil {
		hashFunc = hasher.HashFile
	}
	// #nosec G304 — src — путь из scanner.FileInfo, проверенный перед копированием.
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
			hSrc, err1 := hashFunc(ctx, src)
			hDst, err2 := hashFunc(ctx, dst)
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

	// Проверка целостности: сверяем хеши исходника и копии.
	hSrc, err1 := hashFunc(ctx, src)
	hDst, err2 := hashFunc(ctx, dst)
	if err1 != nil || err2 != nil || hSrc != hDst {
		_ = os.Remove(dst)
		if err1 != nil {
			return fmt.Errorf("%w: source hash error: %v", errIntegrityCheck, err1)
		}
		if err2 != nil {
			return fmt.Errorf("%w: destination hash error: %v", errIntegrityCheck, err2)
		}
		return fmt.Errorf("%w: hash mismatch for %s (src=%x, dst=%x)", errIntegrityCheck, dst, hSrc, hDst)
	}

	return nil
}
