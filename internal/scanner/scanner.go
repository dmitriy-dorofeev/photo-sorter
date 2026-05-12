// Package scanner отвечает за рекурсивный обход исходных папок
// и сбор метаданных о найденных файлах.
package scanner

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// FileInfo содержит метаданные о найденном файле.
type FileInfo struct {
	Path    string
	Name    string
	Size    int64
	ModTime time.Time
	Ext     string
	Device  string // эвристическое определение устройства-источника
}

// Scanner обходит папки и собирает список файлов.
type Scanner struct {
	sources []string
	exts    map[string]struct{}
	skipped []string
}

// New создаёт новый Scanner с заданными исходными папками.
// Если exts не пуст, принимаются только файлы с указанными расширениями
// (регистр неважен).
func New(sources []string, exts ...string) *Scanner {
	s := &Scanner{
		sources: sources,
		exts:    make(map[string]struct{}),
	}
	for _, e := range exts {
		s.exts[strings.ToLower(e)] = struct{}{}
	}
	return s
}

// SkippedPaths возвращает список путей, которые были пропущены во время
// последнего Scan (permission denied, non-regular и т.д.).
func (s *Scanner) SkippedPaths() []string {
	return s.skipped
}

// Scan рекурсивно обходит все исходные папки и возвращает список файлов.
// Обход нескольких папок выполняется параллельно через errgroup.
func (s *Scanner) Scan(ctx context.Context) ([]FileInfo, error) {
	files := make([]FileInfo, 0, 1024)
	s.skipped = nil
	var mu sync.Mutex
	g, ctx := errgroup.WithContext(ctx)

	for _, src := range s.sources {
		src := src
		g.Go(func() error {
			var local []FileInfo
			err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					if d != nil && d.IsDir() {
						// Запоминаем недоступную директорию перед пропуском.
						mu.Lock()
						s.skipped = append(s.skipped, path)
						mu.Unlock()
						return filepath.SkipDir
					}
					// Для корневого пути прокидываем ошибку (например, несуществующая директория).
					if path == src {
						return err
					}
					// Запоминаем пропущенный файл (permission denied и т.п.)
					mu.Lock()
					s.skipped = append(s.skipped, path)
					mu.Unlock()
					return nil
				}
				if d.IsDir() {
					return nil
				}

				// Пропускаем non-regular файлы (symlink, FIFO, device и т.д.)
				// чтобы избежать abort pipeline в deduper/hasher.
				info, err := d.Info()
				if err != nil {
					return nil
				}
				if !info.Mode().IsRegular() {
					return nil
				}

				ext := strings.ToLower(filepath.Ext(d.Name()))
				if !s.allowed(ext) {
					return nil
				}

				local = append(local, FileInfo{
					Path:    path,
					Name:    d.Name(),
					Size:    info.Size(),
					ModTime: info.ModTime(),
					Ext:     ext,
				})
				return nil
			})
			if err != nil {
				return err
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				mu.Lock()
				files = append(files, local...)
				mu.Unlock()
				return nil
			}
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return files, nil
}

func (s *Scanner) allowed(ext string) bool {
	if len(s.exts) == 0 {
		return true
	}
	_, ok := s.exts[ext]
	return ok
}
