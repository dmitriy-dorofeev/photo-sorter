// Package deduper находит дубликаты файлов.
// Алгоритм: группировка по размеру, затем сравнение по хешу.
package deduper

import (
	"photo-sorter/internal/scanner"
)

// Result содержит оригинал и список дубликатов.
type Result struct {
	Original    scanner.FileInfo
	Duplicates  []scanner.FileInfo
}

// Deduper ищет дублирующиеся файлы.
type Deduper struct {
	files []scanner.FileInfo
}

// New создаёт новый Deduper.
func New(files []scanner.FileInfo) *Deduper {
	return &Deduper{files: files}
}

// FindDuplicates возвращает список групп дубликатов.
// TODO: реализовать двухуровневую дедупликацию (размер → хеш).
func (d *Deduper) FindDuplicates() []Result {
	// Заглушка: пока считаем, что дублей нет.
	return nil
}
