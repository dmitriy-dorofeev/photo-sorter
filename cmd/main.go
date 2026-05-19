package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"photo-sorter/internal/collision"
	"photo-sorter/internal/config"
	"photo-sorter/internal/copier"
	"photo-sorter/internal/dateresolver"
	"photo-sorter/internal/notify"
	"photo-sorter/internal/report"
	"photo-sorter/internal/runner"
	"photo-sorter/internal/sorter"
	"photo-sorter/internal/state"
	"photo-sorter/tui"
)

// version встраивается при сборке через -ldflags.
// По умолчанию "dev" для локальной разработки.
var version = "dev"

// stringSlice реализует flag.Value для repeatable флага --source.
type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

type jsonDupGroup struct {
	Original   string   `json:"original"`
	Duplicates []string `json:"duplicates"`
}

type jsonReport struct {
	FilesFound        int            `json:"files_found"`
	WithDate          int            `json:"with_date"`
	UnsortedCount     int            `json:"unsorted_count"`
	DuplicateCount    int            `json:"duplicate_count"`
	Copied            int            `json:"copied"`
	Skipped           int            `json:"skipped"`
	Errors            int            `json:"errors"`
	ExifWrites        int            `json:"exif_writes"`
	ExifFailures      int            `json:"exif_failures"`
	SpotlightWrites   int            `json:"spotlight_writes"`
	SpotlightFailures int            `json:"spotlight_failures"`
	BytesCopied       int64          `json:"bytes_copied"`
	ErrorList         []string       `json:"error_list,omitempty"`
	DuplicateGroups   []jsonDupGroup `json:"duplicate_groups"`
	UnsortedFiles     []string       `json:"unsorted_files"`
}

func main() {
	// Подкоманда update не использует стандартный flag.Parse.
	if len(os.Args) > 1 && os.Args[1] == "update" {
		runUpdate()
		return
	}

	var (
		sources           stringSlice
		target            string
		template          string
		fileNameTemplate  string
		livePhotos        bool
		includeVideo      bool
		dryRun            bool
		useMTime          bool
		dupStrategy       string
		collisionStrategy string
		format            string
		reportFormat      string
		useTUI            bool
		versionFlag       bool
		checkUpdate       bool
		writeExif         bool
		writeSpotlight    bool
		notifyFlag        bool
		fullCheck         bool
		resetState        bool
		concurrency       int
	)

	flag.Var(&sources, "source", "Исходная папка (можно несколько)")
	flag.StringVar(&target, "target", "", "Целевая папка")
	flag.StringVar(&template, "template", config.DefaultTemplate, "Шаблон папок (Go time layout)")
	flag.StringVar(&fileNameTemplate, "name-template", config.DefaultFileNameTemplate, "Шаблон имён файлов")
	flag.BoolVar(&livePhotos, "live-photos", config.DefaultLivePhotos, "Группировать Live Photos")
	flag.BoolVar(&includeVideo, "include-video", config.DefaultIncludeVideo, "Обрабатывать видео")
	flag.BoolVar(&dryRun, "dry-run", true, "Пробный прогон без копирования")
	flag.BoolVar(&useMTime, "use-mtime", config.DefaultUseMTime, "Fallback на дату изменения файла")
	flag.StringVar(&dupStrategy, "dup-strategy", config.DefaultDupStrategy, "Стратегия дедупликации: path | largest | newest | best-meta")
	flag.StringVar(&collisionStrategy, "collision-strategy", config.DefaultCollisionStrategy, "Стратегия конфликтов имён: counter | hash")
	flag.StringVar(&format, "format", "text", "Формат вывода в stdout: text | json")
	flag.StringVar(&reportFormat, "report-format", config.DefaultReportFormat, "Формат файла-отчёта: text | html")
	flag.BoolVar(&useTUI, "tui", true, "Запустить в интерактивном TUI-режиме")
	flag.BoolVar(&versionFlag, "version", false, "Показать версию и выйти")
	flag.BoolVar(&checkUpdate, "check-update", false, "Проверить наличие обновлений")
	flag.BoolVar(&writeExif, "write-exif", config.DefaultWriteExif, "Записывать определённую дату в EXIF (только имя/mtime)")
	flag.BoolVar(&writeSpotlight, "write-spotlight", config.DefaultWriteSpotlight, "Записывать дату съёмки в Spotlight-теги macOS")
	flag.BoolVar(&notifyFlag, "notify", config.DefaultNotify, "Показать системное уведомление по завершении")
	flag.BoolVar(&fullCheck, "full-check", false, "Игнорировать state, пересортировать все файлы")
	flag.BoolVar(&resetState, "reset-state", false, "Удалить state перед запуском")
	flag.IntVar(&concurrency, "concurrency", config.DefaultConcurrency, "Число параллельных потоков копирования (1 = последовательно)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `photo-sorter — организация фотографий по датам съёмки

Использование:
  photo-sorter [флаги]
  photo-sorter update

Основные флаги:
  --source string      Исходная папка (обязательно, можно несколько)
  --target string      Целевая папка (обязательно)

Настройки сортировки:
  --template string        Шаблон папок (default: "2006-01-02")
  --name-template string   Шаблон имён файлов (default: "{original}{ext}")
  --live-photos        Группировать Live Photos (default: true)
  --include-video      Обрабатывать видео (default: true)
  --dry-run            Пробный прогон (default: true)
  --use-mtime          Fallback на дату изменения (default: true)
  --write-exif         Записывать дату в EXIF при копировании (default: false)
  --write-spotlight    Записывать дату съёмки в Spotlight-теги macOS (default: false)
  --dup-strategy       Стратегия дедупликации: path | largest | newest | best-meta (default: path)
  --collision-strategy Стратегия конфликтов имён: counter | hash (default: counter)

Вывод:
  --format string        Формат вывода в stdout: text | json (default: "text")
  --report-format string Формат файла-отчёта: text | html (default: "html")
  --version              Показать версию и выйти

Обновление:
  --check-update       Проверить наличие новой версии
  update               Установить последнюю версию

Примеры:
  photo-sorter --source ./photos --target ./sorted
  photo-sorter --source ./a --source ./b --target ./out --dry-run=false
  photo-sorter --source ./photos --target ./sorted --format=json
  photo-sorter --source ./photos --target ./sorted --name-template "{YYYY}-{MM}-{DD}_{original}{ext}"
`)
	}

	flag.Parse()

	if versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	if checkUpdate {
		runCheckUpdate()
		os.Exit(0)
	}

	exifPath, hasExif := dateresolver.FindExifTool()
	if !hasExif {
		fmt.Fprintln(os.Stderr, "⚠ exiftool не найден в PATH. Видео-метаданные не будут прочитаны, запись EXIF отключена.")
		if writeExif {
			writeExif = false
		}
	}

	cfg := runner.Config{
		Sources:           sources,
		Target:            target,
		Template:          template,
		FileNameTemplate:  fileNameTemplate,
		LivePhotos:        livePhotos,
		IncludeVideo:      includeVideo,
		UseMTime:          useMTime,
		DupStrategy:       dupStrategy,
		CollisionStrategy: collisionStrategy,
		WriteExif:         writeExif,
		WriteSpotlight:    writeSpotlight,
		ExifToolPath:      exifPath,
		FullCheck:         fullCheck,
		DryRun:            dryRun,
		ReportFormat:      reportFormat,
		Concurrency:       concurrency,
	}

	if resetState && cfg.Target != "" {
		if err := state.Reset(cfg.Target); err != nil {
			fmt.Fprintf(os.Stderr, "⚠ не удалось сбросить state: %v\n", err)
		}
	}

	// TUI-режим: если не указаны source/target и -tui не выключен явно
	if useTUI && len(sources) == 0 && target == "" {
		tui.Run(version)
		return
	}

	// CLI-режим
	if err := validateInputs(cfg, format, reportFormat); err != nil {
		fmt.Fprintln(os.Stderr, err)
		flag.Usage()
		os.Exit(1)
	}

	if err := runCLI(cfg, dryRun, format, reportFormat, notifyFlag); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		os.Exit(1)
	}
}

// validateInputs проверяет корректность аргументов CLI.
func validateInputs(cfg runner.Config, format, reportFormat string) error {
	if len(cfg.Sources) == 0 {
		return fmt.Errorf("ошибка: укажите хотя бы одну исходную папку (--source)")
	}
	if cfg.Target == "" {
		return fmt.Errorf("ошибка: укажите целевую папку (--target)")
	}

	// Проверка доступности target для записи
	if err := os.MkdirAll(cfg.Target, 0750); err != nil {
		return fmt.Errorf("ошибка: не удалось создать целевую папку: %w", err)
	}
	tmpFile, err := os.CreateTemp(cfg.Target, ".write-test-*")
	if err != nil {
		return fmt.Errorf("ошибка: целевая папка недоступна для записи: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("ошибка: целевая папка недоступна для записи: %w", err)
	}
	if err := os.Remove(tmpFile.Name()); err != nil {
		// ignore cleanup error
	}

	if cfg.Template == "" {
		return fmt.Errorf("ошибка: шаблон даты не может быть пустым (--template)")
	}

	if format != "text" && format != "json" {
		return fmt.Errorf("ошибка: формат вывода должен быть 'text' или 'json'")
	}

	if reportFormat != "text" && reportFormat != "html" {
		return fmt.Errorf("ошибка: формат отчёта должен быть 'text' или 'html'")
	}

	if cfg.DupStrategy != "path" && cfg.DupStrategy != "largest" && cfg.DupStrategy != "newest" && cfg.DupStrategy != "best-meta" {
		return fmt.Errorf("ошибка: стратегия дедупликации должна быть одной из: path, largest, newest, best-meta")
	}

	if cfg.CollisionStrategy != "counter" && cfg.CollisionStrategy != "hash" {
		return fmt.Errorf("ошибка: стратегия конфликтов имён должна быть одной из: counter, hash")
	}

	for _, src := range cfg.Sources {
		info, err := os.Stat(src)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("ошибка: исходная папка не существует: %s", src)
			}
			return fmt.Errorf("ошибка: исходная папка недоступна: %s", src)
		}
		if !info.IsDir() {
			return fmt.Errorf("ошибка: %s не является директорией", src)
		}
	}

	// Проверка на пересечение source и target (self-copy).
	absTarget, err := filepath.Abs(cfg.Target)
	if err != nil {
		return fmt.Errorf("ошибка: не удалось определить абсолютный путь target: %w", err)
	}
	for _, src := range cfg.Sources {
		absSrc, err := filepath.Abs(src)
		if err != nil {
			return fmt.Errorf("ошибка: не удалось определить абсолютный путь source: %w", err)
		}
		if absSrc == absTarget {
			return fmt.Errorf("ошибка: исходная папка и целевая папка не могут совпадать: %s", src)
		}
		if strings.HasPrefix(absTarget, absSrc+string(filepath.Separator)) {
			return fmt.Errorf("ошибка: целевая папка не может быть внутри исходной: %s", src)
		}
		if strings.HasPrefix(absSrc, absTarget+string(filepath.Separator)) {
			return fmt.Errorf("ошибка: исходная папка не может быть внутри целевой: %s", src)
		}
	}

	return nil
}

// runCLI выполняет pipeline и копирование в CLI-режиме.
func runCLI(cfg runner.Config, dryRun bool, format, reportFormat string, notifyFlag bool) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	res, err := runner.Run(ctx, cfg, func(stage string, current, total int) {
		fmt.Fprintf(os.Stderr, "%s: %d/%d\n", stage, current, total)
	})
	if err != nil {
		return err
	}

	// Гарантируем закрытие state при выходе из функции.
	if res.State != nil {
		defer func() {
			_ = res.State.Close()
		}()
	}

	c := copier.New(dryRun, cfg.Target, collision.Strategy(cfg.CollisionStrategy))
	c.Concurrency = cfg.Concurrency
	c.WriteExif = cfg.WriteExif
	c.ExifToolPath = cfg.ExifToolPath
	stats, err := c.Copy(ctx, res.Entries, nil)

	// Обновляем state после копирования (даже при частичной ошибке).
	if res.State != nil {
		records := make([]state.Record, 0, len(res.Entries))
		for _, e := range res.Entries {
			records = append(records, state.Record{
				SourcePath: e.Source.Path,
				Size:       e.Source.Size,
				ModTime:    e.Source.ModTime,
				FastHash:   res.FastHashes[e.Source.Path],
				FullHash:   res.FullHashes[e.Source.Path],
				TargetPath: e.Target,
			})
		}
		_ = res.State.Update(records)
		_ = res.State.Cleanup(res.AllPaths)
	}

	if err != nil {
		// Выводим частичную статистику перед возвратом ошибки
		if format == "json" {
			printJSONReport(res, stats)
		} else {
			printTextReport(res, stats)
		}
		if !dryRun {
			_ = writeReportFile(cfg, reportFormat, res, stats, err.Error())
		}
		return fmt.Errorf("копирование: %w", err)
	}

	if format == "json" {
		printJSONReport(res, stats)
	} else {
		printTextReport(res, stats)
	}

	if !dryRun {
		if err := writeReportFile(cfg, reportFormat, res, stats, ""); err != nil {
			fmt.Fprintf(os.Stderr, "⚠ не удалось сохранить отчёт: %v\n", err)
		}
	}

	if notifyFlag {
		summary := notify.Summary{
			Total:   len(res.Entries),
			Copied:  stats.Copied,
			Skipped: stats.Skipped,
			Errors:  stats.Errors,
		}
		if err := notify.Send(summary.Title(), summary.Body()); err != nil {
			fmt.Fprintf(os.Stderr, "⚠ не удалось отправить уведомление: %v\n", err)
		}
	}
	return nil
}

// errorStrings преобразует []error в []string для JSON-отчёта.
func errorStrings(errs []error) []string {
	if len(errs) == 0 {
		return nil
	}
	result := make([]string, len(errs))
	for i, e := range errs {
		result[i] = e.Error()
	}
	return result
}

// collectUnsortedFiles возвращает список файлов без даты из entries.
// Если useBase=true — возвращает только имена файлов (для текстового отчёта),
// иначе — полные пути к исходникам (для JSON).
func collectUnsortedFiles(entries []sorter.Entry, useBase bool) []string {
	var files []string
	for _, e := range entries {
		if !e.Skip && sorter.IsUnsorted(e.Target) {
			if useBase {
				files = append(files, filepath.Base(e.Target))
			} else {
				files = append(files, e.Source.Path)
			}
		}
	}
	return files
}

func printTextReport(res runner.Result, stats copier.Stats) {
	st := res.Stats()
	unsortedFiles := collectUnsortedFiles(res.Entries, true)

	fmt.Printf("Найдено файлов:      %d\n", st.Total)
	fmt.Printf("Определено дат:       %d\n", st.WithDate)
	fmt.Printf("Без даты (unsorted):  %d\n", st.Unsorted)
	fmt.Printf("Дубликатов:           %d\n", st.Duplicates)
	fmt.Println()
	fmt.Printf("Скопировано:  %d\n", stats.Copied)
	fmt.Printf("Пропущено:    %d\n", stats.Skipped)
	fmt.Printf("Ошибок:       %d\n", stats.Errors)
	if stats.ExifWrites > 0 {
		fmt.Printf("EXIF записан: %d\n", stats.ExifWrites)
	}
	if stats.ExifFailures > 0 {
		fmt.Printf("Ошибок EXIF:  %d\n", stats.ExifFailures)
	}
	if stats.SpotlightWrites > 0 {
		fmt.Printf("Spotlight тегов: %d\n", stats.SpotlightWrites)
	}
	if stats.SpotlightFailures > 0 {
		fmt.Printf("Ошибок Spotlight: %d\n", stats.SpotlightFailures)
	}
	if stats.BytesCopied > 0 {
		fmt.Printf("Байт:         %d\n", stats.BytesCopied)
	}
	if len(stats.ErrorList) > 0 {
		fmt.Println()
		fmt.Println("Ошибки:")
		for _, e := range stats.ErrorList {
			fmt.Printf("  %s\n", e.Error())
		}
	}

	if len(res.Duplicates) > 0 {
		fmt.Println()
		fmt.Println("Дубликаты:")
		for _, g := range res.Duplicates {
			fmt.Printf("  %s\n", g.Original.Name)
			for _, d := range g.Duplicates {
				fmt.Printf("    → %s\n", d.Name)
			}
		}
	}

	if len(unsortedFiles) > 0 {
		fmt.Println()
		fmt.Println("Unsorted:")
		for _, name := range unsortedFiles {
			fmt.Printf("  %s\n", name)
		}
	}
}

func printJSONReport(res runner.Result, stats copier.Stats) {
	st := res.Stats()
	unsortedFiles := collectUnsortedFiles(res.Entries, false)
	var dupGroups []jsonDupGroup

	for _, g := range res.Duplicates {
		var dups []string
		for _, d := range g.Duplicates {
			dups = append(dups, d.Path)
		}
		dupGroups = append(dupGroups, jsonDupGroup{
			Original:   g.Original.Path,
			Duplicates: dups,
		})
	}

	report := jsonReport{
		FilesFound:        st.Total,
		WithDate:          st.WithDate,
		UnsortedCount:     st.Unsorted,
		DuplicateCount:    st.Duplicates,
		Copied:            stats.Copied,
		Skipped:           stats.Skipped,
		Errors:            stats.Errors,
		ExifWrites:        stats.ExifWrites,
		ExifFailures:      stats.ExifFailures,
		SpotlightWrites:   stats.SpotlightWrites,
		SpotlightFailures: stats.SpotlightFailures,
		BytesCopied:       stats.BytesCopied,
		ErrorList:         errorStrings(stats.ErrorList),
		DuplicateGroups:   dupGroups,
		UnsortedFiles:     unsortedFiles,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка вывода JSON: %v\n", err)
		os.Exit(1)
	}
}

func writeReportFile(cfg runner.Config, reportFormat string, res runner.Result, stats copier.Stats, fatalError string) error {
	var dupGroups []report.DupGroup
	for _, g := range res.Duplicates {
		dups := make([]string, len(g.Duplicates))
		for i, d := range g.Duplicates {
			dups[i] = d.Path
		}
		dupGroups = append(dupGroups, report.DupGroup{
			Original:   g.Original.Path,
			Duplicates: dups,
			Strategy:   cfg.DupStrategy,
		})
	}

	unsortedFiles := collectUnsortedFiles(res.Entries, false)

	rpt := report.Data{
		Sources:           cfg.Sources,
		Target:            cfg.Target,
		FilesFound:        len(res.Files),
		Copied:            stats.Copied,
		Skipped:           stats.Skipped,
		Errors:            stats.Errors,
		IntegrityFailures: stats.IntegrityFailures,
		ExifWrites:        stats.ExifWrites,
		ExifFailures:      stats.ExifFailures,
		SpotlightWrites:   stats.SpotlightWrites,
		SpotlightFailures: stats.SpotlightFailures,
		BytesCopied:       stats.BytesCopied,
		ErrorList:         stats.ErrorList,
		Duplicates:        dupGroups,
		UnsortedFiles:     unsortedFiles,
		FatalError:        fatalError,
	}

	path, err := report.Generate(cfg.Target, reportFormat, rpt)
	if err == nil {
		fmt.Fprintf(os.Stderr, "Отчёт сохранён: %s\n", path)
	}
	return err
}
