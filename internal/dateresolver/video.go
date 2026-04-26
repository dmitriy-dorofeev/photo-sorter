package dateresolver

import (
	"context"
	"encoding/json"
	"os/exec"
	"time"

	"photo-sorter/internal/scanner"
)

// videoTimeout используется в extractVideoDate и extractVideoDates.
// Переопределяется в тестах для ускорения.
var videoTimeout = 30 * time.Second

// isVideo возвращает true для видео-расширений.
// Контракт: scanner.FileInfo.Ext всегда lowercase.
func isVideo(ext string) bool {
	switch ext {
	case ".mov", ".mp4", ".avi", ".mkv":
		return true
	}
	return false
}

// extractVideoDate пытается извлечь дату съёмки из видео-файла через exiftool.
// Приоритет полей: DateTimeOriginal → CreateDate → MediaCreateDate.
// Если exiftool не найден или произошла любая ошибка — возвращает (_, false).
func extractVideoDate(path, exifToolPath string) (time.Time, bool) {
	if exifToolPath == "" {
		exifToolPath = "exiftool"
	}

	ctx, cancel := context.WithTimeout(context.Background(), videoTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, exifToolPath,
		"-DateTimeOriginal", "-CreateDate", "-MediaCreateDate",
		"-json", "--", path,
	)
	out, err := cmd.Output()
	if err != nil {
		// exec.ErrNotFound или любая другая ошибка — молча fallback.
		return time.Time{}, false
	}

	var results []map[string]interface{}
	if err := json.Unmarshal(out, &results); err != nil {
		return time.Time{}, false
	}
	if len(results) == 0 {
		return time.Time{}, false
	}
	meta := results[0]

	for _, key := range []string{"DateTimeOriginal", "CreateDate", "MediaCreateDate"} {
		if v, ok := meta[key].(string); ok && v != "" {
			if t, err := time.Parse("2006:01:02 15:04:05", v); err == nil {
				return t, true
			}
		}
	}

	return time.Time{}, false
}

// extractVideoDates извлекает даты для нескольких видео-файлов одним
// вызовом exiftool. Возвращает мапу путь → время.
func extractVideoDates(files []scanner.FileInfo, exifToolPath string) map[string]time.Time {
	if exifToolPath == "" {
		exifToolPath = "exiftool"
	}
	if len(files) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), videoTimeout)
	defer cancel()

	args := []string{
		"-DateTimeOriginal", "-CreateDate", "-MediaCreateDate",
		"-json", "--",
	}
	for _, f := range files {
		args = append(args, f.Path)
	}

	cmd := exec.CommandContext(ctx, exifToolPath, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var results []map[string]interface{}
	if err := json.Unmarshal(out, &results); err != nil {
		return nil
	}

	result := make(map[string]time.Time, len(results))
	for _, meta := range results {
		sourceFile, _ := meta["SourceFile"].(string)
		if sourceFile == "" {
			continue
		}
		for _, key := range []string{"DateTimeOriginal", "CreateDate", "MediaCreateDate"} {
			if v, ok := meta[key].(string); ok && v != "" {
				if t, err := time.Parse("2006:01:02 15:04:05", v); err == nil {
					result[sourceFile] = t
					break
				}
			}
		}
	}

	return result
}
