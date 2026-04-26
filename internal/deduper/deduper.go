// Package deduper находит дубликаты файлов.
// Алгоритм: группировка по размеру, затем сравнение по xxhash.
package deduper

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"photo-sorter/internal/scanner"
)

// Result содержит оригинал и список дубликатов.
type Result struct {
	Original   scanner.FileInfo
	Duplicates []scanner.FileInfo
}

// Deduper ищет дублирующиеся файлы.
type Deduper struct {
	files      []scanner.FileInfo
	livePhotos bool
}

// New создаёт новый Deduper.
// livePhotos: если true, пары Live Photos (.HEIC + .MOV с одним basename) не считаются дубликатами.
func New(files []scanner.FileInfo, livePhotos bool) *Deduper {
	return &Deduper{files: files, livePhotos: livePhotos}
}

// FindDuplicates возвращает список групп дубликатов.
// Алгоритм:
//  1. Группировка по размеру.
//  2. Для групп с ≥2 файлами вычисляется xxhash.
//  3. Группировка по хешу внутри размерной группы.
//  4. Пары Live Photos (.HEIC + .MOV с одним basename) исключаются из дубликатов.
func (d *Deduper) FindDuplicates(ctx context.Context) ([]Result, error) {
	if len(d.files) == 0 {
		return nil, nil
	}

	// 1. Группировка по размеру.
	sizeGroups := make(map[int64][]scanner.FileInfo)
	for _, f := range d.files {
		sizeGroups[f.Size] = append(sizeGroups[f.Size], f)
	}

	type fileHash struct {
		info scanner.FileInfo
		hash uint64
	}

	var results []Result

	// 2. Обрабатываем только группы с ≥2 файлами.
	for _, group := range sizeGroups {
		if len(group) < 2 {
			continue
		}

		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// 3. Вычисляем хеш для каждого файла в группе.
		var hashed []fileHash
		for _, f := range group {
			h, err := HashFile(f.Path)
			if err != nil {
				return nil, fmt.Errorf("hash file %s: %w", f.Path, err)
			}
			hashed = append(hashed, fileHash{info: f, hash: h})
		}

		// 4. Группировка по хешу.
		hashGroups := make(map[uint64][]fileHash)
		for _, fh := range hashed {
			hashGroups[fh.hash] = append(hashGroups[fh.hash], fh)
		}

		// 5. Формируем Result для групп с ≥2 файлами.
		for _, hashGroup := range hashGroups {
			if len(hashGroup) < 2 {
				continue
			}

			// Детерминированный выбор оригинала (по пути) — избегаем случайности map-итерации.
			sort.Slice(hashGroup, func(i, j int) bool {
				return hashGroup[i].info.Path < hashGroup[j].info.Path
			})

			original := hashGroup[0].info
			var duplicates []scanner.FileInfo

			for i := 1; i < len(hashGroup); i++ {
				candidate := hashGroup[i].info
				if d.livePhotos && isLivePhotoPair(original, candidate) {
					continue
				}
				duplicates = append(duplicates, candidate)
			}

			if len(duplicates) > 0 {
				results = append(results, Result{
					Original:   original,
					Duplicates: duplicates,
				})
			}
		}
	}

	return results, nil
}

// isLivePhotoPair возвращает true, если два файла являются парой Live Photos:
// одинаковый basename (без расширения), один — .heic/.heif, другой — .mov.
func isLivePhotoPair(a, b scanner.FileInfo) bool {
	baseA := strings.TrimSuffix(a.Name, filepath.Ext(a.Name))
	baseB := strings.TrimSuffix(b.Name, filepath.Ext(b.Name))
	if !strings.EqualFold(baseA, baseB) {
		return false
	}

	extA := strings.ToLower(a.Ext)
	extB := strings.ToLower(b.Ext)

	isImage := extA == ".heic" || extA == ".heif"
	isVideo := extB == ".mov"
	if isImage && isVideo {
		return true
	}

	isImage = extB == ".heic" || extB == ".heif"
	isVideo = extA == ".mov"
	if isImage && isVideo {
		return true
	}

	return false
}
