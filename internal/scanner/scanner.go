// Package scanner отвечает за рекурсивный обход исходных папок
// и сбор метаданных о найденных файлах.
package scanner

import (
	"io/fs"
	"path/filepath"
	"time"
)

// FileInfo содержит метаданные о найденном файле.
type FileInfo struct {
	Path       string
	Name       string
	Size       int64
	ModTime    time.Time
	Ext        string
}

// Scanner обходит папки и собирает список файлов.
type Scanner struct {
	sources []string
}

// New создаёт новый Scanner с заданными исходными папками.
func New(sources []string) *Scanner {
	return &Scanner{sources: sources}
}

// Scan рекурсивно обходит все исходные папки и возвращает список файлов.
// TODO: добавить параллельную обработку через errgroup.
func (s *Scanner) Scan() ([]FileInfo, error) {
	var files []FileInfo

	for _, src := range s.sources {
		err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}

			info, err := d.Info()
			if err != nil {
				return err
			}

			files = append(files, FileInfo{
				Path:    path,
				Name:    d.Name(),
				Size:    info.Size(),
				ModTime: info.ModTime(),
				Ext:     filepath.Ext(d.Name()),
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return files, nil
}
