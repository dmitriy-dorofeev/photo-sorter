// Package sorter строит целевую структуру папок на основе дат файлов.
package sorter

import (
	"path/filepath"
	"photo-sorter/internal/scanner"
	"time"
)

// Entry описывает одну операцию копирования.
type Entry struct {
	Source scanner.FileInfo
	Target string // полный целевой путь
	Date   time.Time
}

// Sorter строит план сортировки.
type Sorter struct {
	targetRoot string
	layout     string // например, "2006/01/02"
}

// New создаёт новый Sorter.
func New(targetRoot, layout string) *Sorter {
	return &Sorter{
		targetRoot: targetRoot,
		layout:     layout,
	}
}

// BuildTree строит список операций копирования.
// TODO: обработать коллизии имён и Live Photos.
func (s *Sorter) BuildTree(files []scanner.FileInfo, resolveDate func(scanner.FileInfo) (time.Time, bool)) []Entry {
	var entries []Entry

	for _, f := range files {
		date, ok := resolveDate(f)
		if !ok {
			// Файлы без даты попадают в unsorted/
			entries = append(entries, Entry{
				Source: f,
				Target: filepath.Join(s.targetRoot, "unsorted", f.Name),
			})
			continue
		}

		dir := filepath.Join(s.targetRoot, date.Format(s.layout))
		entries = append(entries, Entry{
			Source: f,
			Target: filepath.Join(dir, f.Name),
			Date:   date,
		})
	}

	return entries
}
