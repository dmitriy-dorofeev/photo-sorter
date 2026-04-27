// Package dateresolver определяет дату съёмки/создания файла
// по приоритету: EXIF/метаданные → имя файла → mtime.
package dateresolver

import (
	"context"
	"photo-sorter/internal/scanner"
	"time"
)

// Resolver определяет дату для файла.
type Resolver struct {
	// UseModTime разрешает fallback на ModTime, если EXIF и имя файла
	// не содержат дату. По умолчанию false — файлы без даты идут в unsorted.
	UseModTime bool
	// ExifToolPath — путь к бинарнику exiftool (по умолчанию "exiftool").
	// Используется для извлечения метаданных из видео.
	ExifToolPath string
	// videoCache заполняется ResolveBatch и используется Resolve
	// для быстрого доступа к датам видео без повторных вызовов exiftool.
	videoCache map[string]time.Time
}

// New создаёт новый Resolver с UseModTime=true.
func New() *Resolver {
	return &Resolver{UseModTime: true}
}

// ResolveBatch предварительно извлекает даты для всех видео-файлов
// одним вызовом exiftool. Результат кэшируется внутри Resolver.
// Поддерживает отмену через ctx.
func (r *Resolver) ResolveBatch(ctx context.Context, files []scanner.FileInfo) {
	var videoFiles []scanner.FileInfo
	for _, f := range files {
		if isVideo(f.Ext) {
			videoFiles = append(videoFiles, f)
		}
	}
	if len(videoFiles) == 0 {
		return
	}
	r.videoCache = extractVideoDates(ctx, videoFiles, r.ExifToolPath)
	if r.videoCache == nil {
		r.videoCache = make(map[string]time.Time)
	}
}

// Resolve возвращает наилучшую возможную дату для файла.
// Приоритет:
//  1. EXIF (DateTimeOriginal / DateTime) для JPEG.
//  2. Видео-метаданные через exiftool (для .mov/.mp4/.avi/.mkv).
//  3. Парсинг имени файла по известным паттернам.
//  4. ModTime файла (только если UseModTime == true).
//
// Если дата не определена ни одним способом — возвращает (_, false).
// ctx используется для прерывания вызова exiftool в fallback-сценарии.
func (r *Resolver) Resolve(ctx context.Context, f scanner.FileInfo) (time.Time, bool) {
	// 1. EXIF для JPEG.
	if isJPEG(f.Ext) {
		if t, ok := extractExifDate(f.Path); ok {
			return t, true
		}
	}

	// 2. Видео-метаданные через exiftool (сначала кэш, затем fallback).
	if isVideo(f.Ext) {
		if t, ok := r.videoCache[f.Path]; ok {
			return t, true
		}
		if t, ok := extractVideoDate(ctx, f.Path, r.ExifToolPath); ok {
			return t, true
		}
	}

	// 3. Парсинг имени файла.
	if t, ok := parseFromFilename(f.Name); ok {
		return t, true
	}

	// 4. Fallback на ModTime (опционально).
	if r.UseModTime && !f.ModTime.IsZero() {
		return f.ModTime, true
	}

	return time.Time{}, false
}

// isJPEG возвращает true для расширений .jpg и .jpeg.
// Контракт: scanner.FileInfo.Ext всегда lowercase.
func isJPEG(ext string) bool {
	return ext == ".jpg" || ext == ".jpeg"
}
