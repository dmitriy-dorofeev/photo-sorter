// Package dateresolver определяет дату съёмки/создания файла
// по приоритету: EXIF/метаданные → имя файла → mtime.
package dateresolver

import (
	"photo-sorter/internal/scanner"
	"strings"
	"time"
)

// Resolver определяет дату для файла.
type Resolver struct {
	// UseModTime разрешает fallback на ModTime, если EXIF и имя файла
	// не содержат дату. По умолчанию false — файлы без даты идут в unsorted.
	UseModTime bool
}

// New создаёт новый Resolver с UseModTime=false.
func New() *Resolver {
	return &Resolver{UseModTime: false}
}

// Resolve возвращает наилучшую возможную дату для файла.
// Приоритет:
//  1. EXIF (DateTimeOriginal / DateTime) для JPEG.
//  2. Парсинг имени файла по известным паттернам.
//  3. ModTime файла (только если UseModTime == true).
//
// Если дата не определена ни одним способом — возвращает (_, false).
func (r *Resolver) Resolve(f scanner.FileInfo) (time.Time, bool) {
	// 1. EXIF для JPEG.
	if isJPEG(f.Ext) {
		if t, ok := extractExifDate(f.Path); ok {
			return t, true
		}
	}

	// 2. Парсинг имени файла.
	if t, ok := parseFromFilename(f.Name); ok {
		return t, true
	}

	// 3. Fallback на ModTime (опционально).
	if r.UseModTime && !f.ModTime.IsZero() {
		return f.ModTime, true
	}

	return time.Time{}, false
}

// isJPEG возвращает true для расширений .jpg и .jpeg (любой регистр).
func isJPEG(ext string) bool {
	e := strings.ToLower(ext)
	return e == ".jpg" || e == ".jpeg"
}
