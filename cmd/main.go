package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"photo-sorter/internal/copier"
	"photo-sorter/internal/runner"
	"photo-sorter/tui"
)

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
	DuplicateGroups []jsonDupGroup `json:"duplicate_groups"`
	UnsortedFiles   []string       `json:"unsorted_files"`
}

func main() {
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
	)

	flag.Var(&sources, "source", "Исходная папка (можно несколько)")
	flag.StringVar(&target, "target", "", "Целевая папка")
	flag.StringVar(&template, "template", "2006/01/02", "Шаблон папок (Go time layout)")
	flag.BoolVar(&livePhotos, "live-photos", true, "Группировать Live Photos")
	flag.BoolVar(&includeVideo, "include-video", true, "Обрабатывать видео")
	flag.BoolVar(&dryRun, "dry-run", true, "Пробный прогон без копирования")
	flag.BoolVar(&useMTime, "use-mtime", false, "Fallback на дату изменения файла")
	flag.StringVar(&format, "format", "text", "Формат отчёта: text | json")
	flag.BoolVar(&useTUI, "tui", true, "Запустить в интерактивном TUI-режиме")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `photo-sorter — организация фотографий по датам съёмки

Использование:
  photo-sorter [флаги]

Основные флаги:
  --source string      Исходная папка (обязательно, можно несколько)
  --target string      Целевая папка (обязательно)

Настройки сортировки:
  --template string    Шаблон папок (default: "2006/01/02")
  --live-photos        Группировать Live Photos (default: true)
  --include-video      Обрабатывать видео (default: true)
  --dry-run            Пробный прогон (default: true)
  --use-mtime          Fallback на дату изменения (default: false)

Вывод:
  --format string      Формат отчёта: text | json (default: "text")

Примеры:
  photo-sorter --source ./photos --target ./sorted
  photo-sorter --source ./a --source ./b --target ./out --dry-run=false
  photo-sorter --source ./photos --target ./sorted --format=json
`)
	}

	flag.Parse()

	// TUI-режим: если не указаны source/target и -tui не выключен явно
	if useTUI && len(sources) == 0 && target == "" {
		tui.Run()
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

	if time.Now().Format(template) == template {
		fmt.Fprintln(os.Stderr, "Ошибка: некорректный шаблон даты (--template)")
		os.Exit(1)
	}

	if format != "text" && format != "json" {
		fmt.Fprintln(os.Stderr, "Ошибка: формат должен быть 'text' или 'json'")
		os.Exit(1)
	}

	for _, src := range sources {
		if _, err := os.Stat(src); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Ошибка: исходная папка не существует: %s\n", src)
			os.Exit(1)
		}
	}

	cfg := runner.Config{
		Sources:      sources,
		Target:       target,
		Template:     template,
		LivePhotos:   livePhotos,
		IncludeVideo: includeVideo,
		DryRun:       dryRun,
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
		os.Exit(1)
	}

	if format == "json" {
		printJSONReport(res, stats)
	} else {
		printTextReport(res, stats)
	}
}

func printTextReport(res runner.Result, stats copier.Stats) {
	total := len(res.Files)
	var withDate, unsortedCount, dupCount int
	var unsortedFiles []string

	for _, e := range res.Entries {
		if e.Skip {
			dupCount++
		} else if strings.Contains(e.Target, "unsorted") {
			unsortedCount++
			unsortedFiles = append(unsortedFiles, filepath.Base(e.Target))
		} else {
			withDate++
		}
	}

	fmt.Printf("Найдено файлов:      %d\n", total)
	fmt.Printf("Определено дат:       %d\n", withDate)
	fmt.Printf("Без даты (unsorted):  %d\n", unsortedCount)
	fmt.Printf("Дубликатов:           %d\n", dupCount)
	fmt.Println()
	fmt.Printf("Скопировано:  %d\n", stats.Copied)
	fmt.Printf("Пропущено:    %d\n", stats.Skipped)
	fmt.Printf("Ошибок:       %d\n", stats.Errors)
	if stats.BytesCopied > 0 {
		fmt.Printf("Байт:         %d\n", stats.BytesCopied)
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
	total := len(res.Files)
	var withDate, unsortedCount, dupCount int
	var unsortedFiles []string
	var dupGroups []jsonDupGroup

	for _, e := range res.Entries {
		if e.Skip {
			dupCount++
		} else if strings.Contains(e.Target, "unsorted") {
			unsortedCount++
			unsortedFiles = append(unsortedFiles, e.Source.Path)
		} else {
			withDate++
		}
	}

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
		FilesFound:      total,
		WithDate:        withDate,
		UnsortedCount:   unsortedCount,
		DuplicateCount:  dupCount,
		Copied:          stats.Copied,
		Skipped:         stats.Skipped,
		Errors:          stats.Errors,
		BytesCopied:     stats.BytesCopied,
		DuplicateGroups: dupGroups,
		UnsortedFiles:   unsortedFiles,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(report)
}
