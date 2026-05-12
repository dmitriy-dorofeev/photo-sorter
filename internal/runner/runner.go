// Package runner выполняет полный pipeline: scan → dedup → sort.
// Используется и TUI, и CLI.
package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"photo-sorter/internal/collision"
	"photo-sorter/internal/dateresolver"
	"photo-sorter/internal/deduper"
	"photo-sorter/internal/renamer"
	"photo-sorter/internal/scanner"
	"photo-sorter/internal/sorter"
)

// Config описывает параметры запуска pipeline.
type Config struct {
	Sources           []string // исходные папки
	Target            string   // целевая папка
	Template          string   // шаблон папок (Go time layout)
	FileNameTemplate  string   // шаблон имён файлов
	LivePhotos        bool     // группировать Live Photos
	IncludeVideo      bool     // включать видео
	UseMTime          bool     // fallback на дату изменения файла
	DupStrategy       string   // стратегия выбора оригинала из дубликатов
	CollisionStrategy string   // стратегия разрешения конфликтов имён
}

// Result содержит результаты этапов pipeline.
type Result struct {
	Files      []scanner.FileInfo
	Duplicates []deduper.Result
	Entries    []sorter.Entry
}

// ResultStats содержит агрегированную статистику по результатам pipeline.
type ResultStats struct {
	Total      int
	WithDate   int
	Unsorted   int
	Duplicates int
}

// Stats вычисляет статистику по Entries: дубликаты, с датой, без даты.
func (r Result) Stats() ResultStats {
	var s ResultStats
	s.Total = len(r.Files)
	for _, e := range r.Entries {
		if e.Skip {
			s.Duplicates++
		} else if sorter.IsUnsorted(e.Target) {
			s.Unsorted++
		} else {
			s.WithDate++
		}
	}
	return s
}

// Run выполняет полный pipeline: сканирование → дедупликация → сортировка.
// progress вызывается после завершения каждого этапа (stage: "scan", "dedup", "sort").
func Run(ctx context.Context, cfg Config, progress func(stage string, current, total int)) (res Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in pipeline: %v", r)
		}
	}()

	// Валидация шаблона даты.
	if strings.Contains(time.Now().Format(cfg.Template), "%!") {
		return res, fmt.Errorf("invalid date template: %s", cfg.Template)
	}

	// Валидация и парсинг шаблона имён файлов.
	fileNameTmpl, err := renamer.Parse(cfg.FileNameTemplate)
	if err != nil {
		return res, fmt.Errorf("invalid file name template: %w", err)
	}

	exts := []string{".jpg", ".jpeg", ".png", ".heic", ".heif"}
	if cfg.IncludeVideo {
		exts = append(exts, ".mov", ".mp4", ".avi", ".mkv")
	}

	// 1. Scan
	sc := scanner.New(cfg.Sources, exts...)
	files, err := sc.Scan(ctx)
	if err != nil {
		return res, fmt.Errorf("scan: %w", err)
	}

	// Заполняем Device для каждого файла.
	for i := range files {
		files[i].Device = renamer.DetectDevice(files[i].Name)
	}

	res.Files = files
	if progress != nil {
		progress("scan", len(files), len(files))
	}

	// 2. Date resolve (batch для видео) + собираем источники дат.
	dr := dateresolver.New()
	dr.UseModTime = cfg.UseMTime
	dr.ResolveBatch(ctx, files)

	dateSources := make(map[string]dateresolver.Source, len(files))
	for _, f := range files {
		_, _, src := dr.ResolveWithSource(ctx, f)
		dateSources[f.Path] = src
	}

	// 3. Dedup
	strategy := deduper.Strategy(cfg.DupStrategy)
	if strategy == "" {
		strategy = deduper.StrategyPath
	}
	d := deduper.New(files, cfg.LivePhotos, strategy, dateSources)
	res.Duplicates, err = d.FindDuplicates(ctx)
	if err != nil {
		return res, fmt.Errorf("dedup: %w", err)
	}
	if progress != nil {
		progress("dedup", len(res.Duplicates), len(res.Duplicates))
	}

	// 4. Sort
	srt := sorter.New(cfg.Target, cfg.Template, cfg.LivePhotos, fileNameTmpl, collision.Strategy(cfg.CollisionStrategy))
	res.Entries = srt.BuildTree(ctx, files, res.Duplicates, dr.Resolve)
	if progress != nil {
		progress("sort", len(res.Entries), len(res.Entries))
	}

	return res, err
}
