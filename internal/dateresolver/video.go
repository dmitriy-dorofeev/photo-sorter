package dateresolver

import (
	"encoding/json"
	"os/exec"
	"time"
)

// isVideo возвращает true для видео-расширений.
func isVideo(ext string) bool {
	switch ext {
	case ".mov", ".mp4", ".avi", ".mkv",
		".MOV", ".MP4", ".AVI", ".MKV":
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

	cmd := exec.Command(exifToolPath,
		"-DateTimeOriginal", "-CreateDate", "-MediaCreateDate",
		"-json", path,
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
