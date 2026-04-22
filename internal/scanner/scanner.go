// Package scanner отвечает за рекурсивный обход исходных папок
// и сбор метаданных о найденных файлах.
package scanner

import (
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
}

// Scanner обходит папки и собирает список файлов.
type Scanner struct {
	sources []string
	exts    map[string]struct{}
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

// Scan рекурсивно обходит все исходные папки и возвращает список файлов.
// Обход нескольких папок выполняется параллельно через errgroup.
func (s *Scanner) Scan() ([]FileInfo, error) {
	var mu sync.Mutex
	var files []FileInfo
	g := new(errgroup.Group)

	for _, src := range s.sources {
		src := src
		g.Go(func() error {
			return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}

				ext := strings.ToLower(filepath.Ext(d.Name()))
				if !s.allowed(ext) {
					return nil
				}

				info, err := d.Info()
				if err != nil {
					return err
				}

				mu.Lock()
				files = append(files, FileInfo{
					Path:    path,
					Name:    d.Name(),
					Size:    info.Size(),
					ModTime: info.ModTime(),
					Ext:     ext,
				})
				mu.Unlock()
				return nil
			})
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
