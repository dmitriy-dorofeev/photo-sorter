// Package runner выполняет полный pipeline: scan → dedup → sort.
// Используется и TUI, и CLI.
package runner

import (
	"context"
	"fmt"

	"photo-sorter/internal/dateresolver"
	"photo-sorter/internal/deduper"
	"photo-sorter/internal/scanner"
	"photo-sorter/internal/sorter"
)

// Config описывает параметры запуска pipeline.
type Config struct {
	Sources      []string // исходные папки
	Target       string   // целевая папка
	Template     string   // шаблон папок (Go time layout)
	LivePhotos   bool     // группировать Live Photos
	IncludeVideo bool     // включать видео
	DryRun       bool     // пробный прогон (не используется в runner, нужен для CLI)
	UseMTime     bool     // fallback на дату изменения файла
}

// Result содержит результаты этапов pipeline.
type Result struct {
	Files      []scanner.FileInfo
	Duplicates []deduper.Result
	Entries    []sorter.Entry
}

// Run выполняет полный pipeline: сканирование → дедупликация → сортировка.
// progress вызывается после завершения каждого этапа (stage: "scan", "dedup", "sort").
func Run(ctx context.Context, cfg Config, progress func(stage string, current, total int)) (Result, error) {
	var res Result

	exts := []string{".jpg", ".jpeg", ".png", ".heic", ".heif"}
	if cfg.IncludeVideo {
		exts = append(exts, ".mov", ".mp4", ".avi", ".mkv")
	}

	// 1. Scan
	sc := scanner.New(cfg.Sources, exts...)
	files, err := sc.Scan()
	if err != nil {
		return res, fmt.Errorf("scan: %w", err)
	}
	res.Files = files
	if progress != nil {
		progress("scan", len(files), len(files))
	}

	// 2. Dedup
	d := deduper.New(files, cfg.LivePhotos)
	res.Duplicates = d.FindDuplicates()
	if progress != nil {
		progress("dedup", len(res.Duplicates), len(res.Duplicates))
	}

	// 3. Date resolve + Sort
	dr := dateresolver.New()
	dr.UseModTime = cfg.UseMTime
	sort := sorter.New(cfg.Target, cfg.Template, cfg.LivePhotos)
	res.Entries = sort.BuildTree(files, res.Duplicates, dr.Resolve)
	if progress != nil {
		progress("sort", len(res.Entries), len(res.Entries))
	}

	return res, nil
}
