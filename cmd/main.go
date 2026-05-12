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

	"photo-sorter/internal/config"
	"photo-sorter/internal/copier"
	"photo-sorter/internal/runner"
	"photo-sorter/internal/sorter"
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
	FilesFound      int            `json:"files_found"`
	WithDate        int            `json:"with_date"`
	UnsortedCount   int            `json:"unsorted_count"`
	DuplicateCount  int            `json:"duplicate_count"`
	Copied          int            `json:"copied"`
	Skipped         int            `json:"skipped"`
	Errors          int            `json:"errors"`
	BytesCopied     int64          `json:"bytes_copied"`
	ErrorList       []string       `json:"error_list,omitempty"`
	DuplicateGroups []jsonDupGroup `json:"duplicate_groups"`
	UnsortedFiles   []string       `json:"unsorted_files"`
}

func main() {
	// Подкоманда update не использует стандартный flag.Parse.
	if len(os.Args) > 1 && os.Args[1] == "update" {
		runUpdate()
		return
	}

	var (
		sources      stringSlice
		target       string
		template     string
		livePhotos   bool
		includeVideo bool
		dryRun       bool
		useMTime     bool
		dupStrategy  string
		format       string
		useTUI       bool
		versionFlag  bool
		checkUpdate  bool
	)

	flag.Var(&sources, "source", "Исходная папка (можно несколько)")
	flag.StringVar(&target, "target", "", "Целевая папка")
	flag.StringVar(&template, "template", config.DefaultTemplate, "Шаблон папок (Go time layout)")
	flag.BoolVar(&livePhotos, "live-photos", config.DefaultLivePhotos, "Группировать Live Photos")
	flag.BoolVar(&includeVideo, "include-video", config.DefaultIncludeVideo, "Обрабатывать видео")
	flag.BoolVar(&dryRun, "dry-run", true, "Пробный прогон без копирования")
	flag.BoolVar(&useMTime, "use-mtime", config.DefaultUseMTime, "Fallback на дату изменения файла")
	flag.StringVar(&dupStrategy, "dup-strategy", config.DefaultDupStrategy, "Стратегия дедупликации: path | largest | newest | best-meta")
	flag.StringVar(&format, "format", "text", "Формат отчёта: text | json")
	flag.BoolVar(&useTUI, "tui", true, "Запустить в интерактивном TUI-режиме")
	flag.BoolVar(&versionFlag, "version", false, "Показать версию и выйти")
	flag.BoolVar(&checkUpdate, "check-update", false, "Проверить наличие обновлений")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `photo-sorter — организация фотографий по датам съёмки

Использование:
  photo-sorter [флаги]
  photo-sorter update

Основные флаги:
  --source string      Исходная папка (обязательно, можно несколько)
  --target string      Целевая папка (обязательно)

Настройки сортировки:
  --template string    Шаблон папок (default: "2006-01-02")
  --live-photos        Группировать Live Photos (default: true)
  --include-video      Обрабатывать видео (default: true)
  --dry-run            Пробный прогон (default: true)
  --use-mtime          Fallback на дату изменения (default: true)
  --dup-strategy       Стратегия дедупликации: path | largest | newest | best-meta (default: path)

Вывод:
  --format string      Формат отчёта: text | json (default: "text")
  --version            Показать версию и выйти

Обновление:
  --check-update       Проверить наличие новой версии
  update               Установить последнюю версию

Примеры:
  photo-sorter --source ./photos --target ./sorted
  photo-sorter --source ./a --source ./b --target ./out --dry-run=false
  photo-sorter --source ./photos --target ./sorted --format=json
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

	cfg := runner.Config{
		Sources:      sources,
		Target:       target,
		Template:     template,
		LivePhotos:   livePhotos,
		IncludeVideo: includeVideo,
		UseMTime:     useMTime,
		DupStrategy:  dupStrategy,
	}

	// TUI-режим: если не указаны source/target и -tui не выключен явно
	if useTUI && len(sources) == 0 && target == "" {
		tui.Run(version)
		return
	}

	// CLI-режим
	if err := validateInputs(cfg, format); err != nil {
		fmt.Fprintln(os.Stderr, err)
		flag.Usage()
		os.Exit(1)
	}

	if err := runCLI(cfg, dryRun, format); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		os.Exit(1)
	}
}

// validateInputs проверяет корректность аргументов CLI.
func validateInputs(cfg runner.Config, format string) error {
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
		return fmt.Errorf("ошибка: формат должен быть 'text' или 'json'")
	}

	if cfg.DupStrategy != "path" && cfg.DupStrategy != "largest" && cfg.DupStrategy != "newest" && cfg.DupStrategy != "best-meta" {
		return fmt.Errorf("ошибка: стратегия дедупликации должна быть одной из: path, largest, newest, best-meta")
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
func runCLI(cfg runner.Config, dryRun bool, format string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	res, err := runner.Run(ctx, cfg, func(stage string, current, total int) {
		fmt.Fprintf(os.Stderr, "%s: %d/%d\n", stage, current, total)
	})
	if err != nil {
		return err
	}

	c := copier.New(dryRun, cfg.Target)
	stats, err := c.Copy(ctx, res.Entries, nil)
	if err != nil {
		// Выводим частичную статистику перед возвратом ошибки
		if format == "json" {
			printJSONReport(res, stats)
		} else {
			printTextReport(res, stats)
		}
		return fmt.Errorf("копирование: %w", err)
	}

	if format == "json" {
		printJSONReport(res, stats)
	} else {
		printTextReport(res, stats)
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
		FilesFound:      st.Total,
		WithDate:        st.WithDate,
		UnsortedCount:   st.Unsorted,
		DuplicateCount:  st.Duplicates,
		Copied:          stats.Copied,
		Skipped:         stats.Skipped,
		Errors:          stats.Errors,
		BytesCopied:     stats.BytesCopied,
		ErrorList:       errorStrings(stats.ErrorList),
		DuplicateGroups: dupGroups,
		UnsortedFiles:   unsortedFiles,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка вывода JSON: %v\n", err)
		os.Exit(1)
	}
}
