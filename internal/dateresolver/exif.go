package dateresolver

import (
	"os"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

// extractExifDate пытается извлечь дату съёмки из EXIF файла.
// Поддерживает только JPEG. При любой ошибке возвращает (_, false).
func extractExifDate(path string) (time.Time, bool) {
	// #nosec G304 — путь получен из доверенного scanner, проходит валидацию.
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, false
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		return time.Time{}, false
	}

	tm, err := x.DateTime()
	if err != nil {
		return time.Time{}, false
	}

	return tm, true
}
