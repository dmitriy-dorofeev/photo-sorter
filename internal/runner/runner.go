// Package runner выполняет полный pipeline: scan → state filter → dedup → sort.
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
	"photo-sorter/internal/hasher"
	"photo-sorter/internal/renamer"
	"photo-sorter/internal/scanner"
	"photo-sorter/internal/sorter"
	"photo-sorter/internal/state"
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
	WriteExif         bool     // записывать дату в EXIF при копировании
	ExifToolPath      string   // путь к exiftool (пусто — не использовать)
	FullCheck         bool     // игнорировать state при фильтрации
	DryRun            bool     // пробный прогон — не изменять state
	ReportFormat      string   // формат файла-отчёта: text | html
}

// Result содержит результаты этапов pipeline.
type Result struct {
	Files      []scanner.FileInfo // ВСЕ отсканированные файлы
	Duplicates []deduper.Result
	Entries    []sorter.Entry // только toProcess

	// Данные для инкрементальности.
	AllPaths   []string          // все пути (для state.Cleanup)
	FastHashes map[string]uint64 // fasthash для toProcess
	FullHashes map[string]uint64 // fullhash для toProcess
	State      *state.State      // открытое состояние (nil если FullCheck или ошибка)
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

// Run выполняет полный pipeline: сканирование → фильтрация state → дедупликация → сортировка.
// progress вызывается после завершения каждого этапа (stage: "scan", "dedup", "sort").
// Вызывающий код ОБЯЗАН закрыть res.State (если не nil) после копирования и обновления state.
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
	allPaths := make([]string, len(files))
	for i, f := range files {
		allPaths[i] = f.Path
	}
	res.AllPaths = allPaths

	if progress != nil {
		progress("scan", len(files), len(files))
	}

	// 2. State filter
	var toProcess []scanner.FileInfo
	var unchanged []state.Record
	var knownHashes map[uint64]struct{}
	var st *state.State

	if !cfg.FullCheck && cfg.Target != "" && !cfg.DryRun {
		st, err = state.Open(cfg.Target)
		if err != nil {
			// Не прерываем pipeline — работаем без state.
			st = nil
		} else {
			toProcess, unchanged, err = st.Filter(files)
			if err != nil {
				st.Close()
				st = nil
				toProcess = files
			} else {
				knownHashes = make(map[uint64]struct{})
				for _, rec := range unchanged {
					if rec.FullHash != 0 {
						knownHashes[rec.FullHash] = struct{}{}
					}
				}
			}
		}
	}

	if cfg.FullCheck || st == nil {
		toProcess = files
	}

	// 3. FastHash для всех toProcess файлов.
	fastHashes := make(map[string]uint64, len(toProcess))
	for _, f := range toProcess {
		h, err := hasher.FastHash(f.Path)
		if err == nil {
			fastHashes[f.Path] = h
		}
	}
	res.FastHashes = fastHashes

	// 4. Межзапусковая верификация через FastHash → FullHash.
	crossRunSkip := make(map[string]struct{})
	if st != nil {
		for _, f := range toProcess {
			if _, ok := crossRunSkip[f.Path]; ok {
				continue
			}
			recs, err := st.RecordsBySize(f.Size)
			if err != nil {
				continue
			}
			fh := fastHashes[f.Path]
			for _, rec := range recs {
				if rec.SourcePath == f.Path {
					continue
				}
				if rec.FastHash == 0 || rec.FastHash != fh {
					continue
				}
				// Совпадение FastHash — проверяем FullHash.
				hNew, err1 := hasher.HashFile(ctx, f.Path)
				hOld, err2 := hasher.HashFile(ctx, rec.TargetPath)
				if err1 != nil || err2 != nil {
					continue
				}
				if hNew == hOld {
					crossRunSkip[f.Path] = struct{}{}
					// Обновляем старую запись, если FullHash отсутствовал.
					if rec.FullHash == 0 {
						rec.FullHash = hOld
						_ = st.Update([]state.Record{rec})
					}
					break
				}
			}
		}
	}

	// 5. Date resolve (batch для видео) + собираем источники дат.
	dr := dateresolver.New()
	dr.UseModTime = cfg.UseMTime
	dr.ExifToolPath = cfg.ExifToolPath
	dr.ResolveBatch(ctx, toProcess)

	dateSources := make(map[string]dateresolver.Source, len(toProcess))
	for _, f := range toProcess {
		_, _, src := dr.ResolveWithSource(ctx, f)
		dateSources[f.Path] = src
	}

	// 6. Dedup
	strategy := deduper.Strategy(cfg.DupStrategy)
	if strategy == "" {
		strategy = deduper.StrategyPath
	}
	d := deduper.New(toProcess, cfg.LivePhotos, strategy, dateSources, knownHashes)
	dupResults, crossRunDups, err := d.FindDuplicates(ctx)
	if err != nil {
		return res, fmt.Errorf("dedup: %w", err)
	}
	res.Duplicates = dupResults
	res.FullHashes = d.FileHashes()

	// Объединяем cross-run дубликаты из FastHash→FullHash и из deduper.
	for _, p := range crossRunDups {
		crossRunSkip[p] = struct{}{}
	}

	if progress != nil {
		progress("dedup", len(dupResults), len(dupResults))
	}

	// 7. Sort
	srt := sorter.New(cfg.Target, cfg.Template, cfg.LivePhotos, fileNameTmpl, collision.Strategy(cfg.CollisionStrategy))
	entries := srt.BuildTree(ctx, toProcess, dupResults, dr.Resolve, dateSources)

	// Помечаем межзапусковые дубликаты как Skip.
	for i := range entries {
		if _, ok := crossRunSkip[entries[i].Source.Path]; ok {
			entries[i].Skip = true
		}
	}
	res.Entries = entries

	if progress != nil {
		progress("sort", len(entries), len(entries))
	}

	res.State = st
	return res, nil
}
