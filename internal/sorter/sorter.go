// Package sorter строит целевую структуру папок на основе дат файлов.
package sorter

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"photo-sorter/internal/collision"
	"photo-sorter/internal/deduper"
	"photo-sorter/internal/renamer"
	"photo-sorter/internal/scanner"
)

// UnsortedDir — имя директории для файлов без распознанной даты.
const UnsortedDir = "unsorted"

// IsUnsorted возвращает true, если target находится в директории unsorted.
func IsUnsorted(target string) bool {
	return filepath.Base(filepath.Dir(target)) == UnsortedDir
}

// Entry описывает одну операцию копирования.
type Entry struct {
	Source scanner.FileInfo
	Target string // полный целевой путь
	Date   time.Time
	Skip   bool // true — дубликат, не копировать
}

// Sorter строит план сортировки.
type Sorter struct {
	targetRoot        string
	layout            string // например, "2006/01/02"
	livePhotos        bool
	fileNameTemplate  *renamer.Template
	collisionStrategy collision.Strategy
}

// New создаёт новый Sorter.
// livePhotos: если true, .MOV без даты получает дату от соответствующего .HEIC/.HEIF.
// fileNameTemplate может быть nil — тогда используется {original}{ext}.
// collisionStrategy может быть пустой — тогда используется counter.
func New(targetRoot, layout string, livePhotos bool, fileNameTemplate *renamer.Template, collisionStrategy collision.Strategy) *Sorter {
	if fileNameTemplate == nil {
		fileNameTemplate, _ = renamer.Parse("{original}{ext}")
	}
	if collisionStrategy == "" {
		collisionStrategy = collision.StrategyCounter
	}
	return &Sorter{
		targetRoot:        targetRoot,
		layout:            layout,
		livePhotos:        livePhotos,
		fileNameTemplate:  fileNameTemplate,
		collisionStrategy: collisionStrategy,
	}
}

// BuildTree строит список операций копирования.
//
// Алгоритм:
//  1. Помечает дубликаты (из duplicates) как Skip.
//  2. Разрешает коллизии имён внутри плана: _1, _2, …
//  3. Live Photos: .MOV без даты получает дату от .HEIC/.HEIF с тем же basename.
func (s *Sorter) BuildTree(
	ctx context.Context,
	files []scanner.FileInfo,
	duplicates []deduper.Result,
	resolveDate func(context.Context, scanner.FileInfo) (time.Time, bool),
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
		if ctx.Err() != nil {
			break
		}
		d, ok := resolveDate(ctx, f)
		dateCache[f.Path] = dateResult{date: d, ok: ok}
		if ok && s.livePhotos {
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
	seqCounts := make(map[string]int)    // счётчик {seq} внутри каждой директории

	for _, f := range files {
		if ctx.Err() != nil {
			break
		}
		res := dateCache[f.Path]
		date, ok := res.date, res.ok

		// Live Photos fallback для .MOV
		if !ok && s.livePhotos {
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
		var dir string
		if !ok {
			dir = filepath.Join(s.targetRoot, UnsortedDir)
		} else {
			dir = filepath.Join(s.targetRoot, date.Format(s.layout))
		}

		// Генерируем имя файла по шаблону.
		seqCounts[dir]++
		fileName := s.fileNameTemplate.Render(date, f, seqCounts[dir])
		target = filepath.Join(dir, fileName)

		// Разрешение внутренних коллизий.
		if _, exists := targetCounts[target]; exists {
			for i := 0; ; i++ {
				candidate := collision.Resolve(target, s.collisionStrategy, f.Path, i)
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
