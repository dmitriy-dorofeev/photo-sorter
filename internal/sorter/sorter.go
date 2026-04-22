// Package sorter строит целевую структуру папок на основе дат файлов.
package sorter

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"photo-sorter/internal/deduper"
	"photo-sorter/internal/scanner"
)

// Entry описывает одну операцию копирования.
type Entry struct {
	Source scanner.FileInfo
	Target string // полный целевой путь
	Date   time.Time
	Skip   bool // true — дубликат, не копировать
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
//
// Алгоритм:
//  1. Помечает дубликаты (из duplicates) как Skip.
//  2. Разрешает коллизии имён внутри плана: _1, _2, …
//  3. Live Photos: .MOV без даты получает дату от .HEIC/.HEIF с тем же basename.
func (s *Sorter) BuildTree(
	files []scanner.FileInfo,
	duplicates []deduper.Result,
	resolveDate func(scanner.FileInfo) (time.Time, bool),
) []Entry {
	// 1. Множество путей-дубликатов.
	dupPaths := make(map[string]struct{})
	for _, r := range duplicates {
		for _, d := range r.Duplicates {
			dupPaths[d.Path] = struct{}{}
		}
	}

	// 2. Кеш дат + Live Photos pre-pass.
	type dateResult struct {
		date time.Time
		ok   bool
	}
	dateCache := make(map[string]dateResult, len(files))
	livePhotoDates := make(map[string]time.Time)

	for _, f := range files {
		d, ok := resolveDate(f)
		dateCache[f.Path] = dateResult{date: d, ok: ok}
		if ok {
			ext := strings.ToLower(f.Ext)
			if ext == ".heic" || ext == ".heif" {
				base := strings.TrimSuffix(f.Name, filepath.Ext(f.Name))
				livePhotoDates[strings.ToLower(base)] = d
			}
		}
	}

	// 3. Строим entries.
	var entries []Entry
	targetCounts := make(map[string]int) // для разрешения коллизий

	for _, f := range files {
		res := dateCache[f.Path]
		date, ok := res.date, res.ok

		// Live Photos fallback для .MOV
		if !ok {
			ext := strings.ToLower(f.Ext)
			if ext == ".mov" {
				base := strings.TrimSuffix(f.Name, filepath.Ext(f.Name))
				if d, found := livePhotoDates[strings.ToLower(base)]; found {
					date = d
					ok = true
				}
			}
		}

		var target string
		if !ok {
			target = filepath.Join(s.targetRoot, "unsorted", f.Name)
		} else {
			dir := filepath.Join(s.targetRoot, date.Format(s.layout))
			target = filepath.Join(dir, f.Name)
		}

		// Разрешение внутренних коллизий.
		if _, exists := targetCounts[target]; exists {
			dir := filepath.Dir(target)
			ext := filepath.Ext(target)
			base := strings.TrimSuffix(filepath.Base(target), ext)
			for i := 1; ; i++ {
				candidate := filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, i, ext))
				if _, taken := targetCounts[candidate]; !taken {
					target = candidate
					break
				}
			}
		}
		targetCounts[target]++

		entry := Entry{
			Source: f,
			Target: target,
			Date:   date,
			Skip:   false,
		}
		if _, isDup := dupPaths[f.Path]; isDup {
			entry.Skip = true
		}
		entries = append(entries, entry)
	}

	return entries
}
