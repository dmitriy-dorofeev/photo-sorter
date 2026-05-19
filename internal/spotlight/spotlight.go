// Package spotlight предоставляет API для записи метаданных Spotlight на macOS.
package spotlight

import "time"

// Available возвращает true, если запись Spotlight-атрибутов поддерживается на текущей платформе.
func Available() bool {
	return available()
}

// WriteTags записывает дату съёмки в расширенные атрибуты файла,
// чтобы Spotlight и Finder могли индексировать файл по дате.
func WriteTags(path string, date time.Time) error {
	return writeTags(path, date)
}
