// Package dateresolver определяет дату съёмки/создания файла
// по приоритету: EXIF/метаданные → имя файла → mtime.
package dateresolver

import (
	"path/filepath"
	"photo-sorter/internal/scanner"
	"time"
)

// Resolver определяет дату для файла.
type Resolver struct{}

// New создаёт новый Resolver.
func New() *Resolver {
	return &Resolver{}
}

// Resolve возвращает наилучшую возможную дату для файла.
// TODO: добавить чтение EXIF и парсинг имён файлов.
func (r *Resolver) Resolve(f scanner.FileInfo) (time.Time, bool) {
	// 1. Попытка извлечь дату из EXIF или видео-метаданных.
	// 2. Попытка распарсить имя файла по известным паттернам.
	// 3. Fallback на ModTime.
	_ = f.Name // временно, чтобы компилятор не ругался
	_ = filepath.Ext(f.Name)
	return f.ModTime, true
}
