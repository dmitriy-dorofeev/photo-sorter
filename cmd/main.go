package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
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

	// TUI-режим: если не указаны source/target и -tui не выключен явно
	if useTUI && len(sources) == 0 && target == "" {
		tui.Run(version)
		return
	}

	// CLI-режим
	if len(sources) == 0 {
		fmt.Fprintln(os.Stderr, "Ошибка: укажите хотя бы одну исходную папку (--source)")
		flag.Usage()
		os.Exit(1)
	}
	if target == "" {
		fmt.Fprintln(os.Stderr, "Ошибка: укажите целевую папку (--target)")
		flag.Usage()
		os.Exit(1)
	}

	// Проверка доступности target для записи
	if err := os.MkdirAll(target, 0750); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: не удалось создать целевую папку: %v\n", err)
		os.Exit(1)
	}
	tmpFile, err := os.CreateTemp(target, ".write-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: целевая папка недоступна для записи: %v\n", err)
		os.Exit(1)
	}
	tmpFile.Close()
	os.Remove(tmpFile.Name())

	if template == "" {
		fmt.Fprintln(os.Stderr, "Ошибка: шаблон даты не может быть пустым (--template)")
		os.Exit(1)
	}

	if format != "text" && format != "json" {
		fmt.Fprintln(os.Stderr, "Ошибка: формат должен быть 'text' или 'json'")
		os.Exit(1)
	}

	for _, src := range sources {
		info, err := os.Stat(src)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Ошибка: исходная папка не существует: %s\n", src)
			} else {
				fmt.Fprintf(os.Stderr, "Ошибка: исходная папка недоступна: %s\n", src)
			}
			os.Exit(1)
		}
		if !info.IsDir() {
			fmt.Fprintf(os.Stderr, "Ошибка: %s не является директорией\n", src)
			os.Exit(1)
		}
	}

	// Проверка на пересечение source и target (self-copy).
	absTarget, err := filepath.Abs(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: не удалось определить абсолютный путь target: %v\n", err)
		os.Exit(1)
	}
	for _, src := range sources {
		absSrc, err := filepath.Abs(src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Ошибка: не удалось определить абсолютный путь source: %v\n", err)
			os.Exit(1)
		}
		if absSrc == absTarget {
			fmt.Fprintf(os.Stderr, "Ошибка: исходная папка и целевая папка не могут совпадать: %s\n", src)
			os.Exit(1)
		}
		if strings.HasPrefix(absTarget, absSrc+string(filepath.Separator)) {
			fmt.Fprintf(os.Stderr, "Ошибка: целевая папка не может быть внутри исходной: %s\n", src)
			os.Exit(1)
		}
		if strings.HasPrefix(absSrc, absTarget+string(filepath.Separator)) {
			fmt.Fprintf(os.Stderr, "Ошибка: исходная папка не может быть внутри целевой: %s\n", src)
			os.Exit(1)
		}
	}

	cfg := runner.Config{
		Sources:      sources,
		Target:       target,
		Template:     template,
		LivePhotos:   livePhotos,
		IncludeVideo: includeVideo,
		UseMTime:     useMTime,
	}

	res, err := runner.Run(context.Background(), cfg, func(stage string, current, total int) {
		fmt.Fprintf(os.Stderr, "%s: %d/%d\n", stage, current, total)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		os.Exit(1)
	}

	// Копирование
	c := copier.New(dryRun, target)
	stats, err := c.Copy(context.Background(), res.Entries, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка копирования: %v\n", err)
		// Выводим частичную статистику перед выходом
		if format == "json" {
			printJSONReport(res, stats)
		} else {
			printTextReport(res, stats)
		}
		os.Exit(1)
	}

	if format == "json" {
		printJSONReport(res, stats)
	} else {
		printTextReport(res, stats)
	}
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
