// Package deduper находит дубликаты файлов.
// Алгоритм: группировка по размеру, затем сравнение по xxhash.
package deduper

import (
	"context"
	"path/filepath"
	"strings"

	"photo-sorter/internal/dateresolver"
	"photo-sorter/internal/hasher"
	"photo-sorter/internal/scanner"
)

// Result содержит оригинал и список дубликатов.
type Result struct {
	Original   scanner.FileInfo
	Duplicates []scanner.FileInfo
	Hash       uint64 // xxhash группы дубликатов
}

// Deduper ищет дублирующиеся файлы.
type Deduper struct {
	files             []scanner.FileInfo
	livePhotos        bool
	rawJPEGClustering bool
	strategy          Strategy
	dateSources       map[string]dateresolver.Source
	knownHashes       map[uint64]struct{} // FullHash из state (межзапусковые)
	hashes            map[string]uint64   // все вычисленные FullHash (путь → хеш)
}

// New создаёт новый Deduper.
// livePhotos: если true, пары Live Photos (.HEIC + .MOV с одним basename) не считаются дубликатами.
// rawJPEGClustering: если true, пары RAW + JPEG с одинаковым basename не считаются дубликатами.
// strategy: стратегия выбора оригинала из группы дубликатов.
// dateSources: мапа путь → источник даты (используется для стратегии best-meta).
// knownHashes: FullHash из state для межзапусковой дедупликации.
func New(files []scanner.FileInfo, livePhotos bool, rawJPEGClustering bool, strategy Strategy, dateSources map[string]dateresolver.Source, knownHashes map[uint64]struct{}) *Deduper {
	if knownHashes == nil {
		knownHashes = make(map[uint64]struct{})
	}
	return &Deduper{
		files:             files,
		livePhotos:        livePhotos,
		rawJPEGClustering: rawJPEGClustering,
		strategy:          strategy,
		dateSources:       dateSources,
		knownHashes:       knownHashes,
		hashes:            make(map[string]uint64),
	}
}

// FileHashes возвращает мапу всех вычисленных FullHash (путь → хеш).
func (d *Deduper) FileHashes() map[string]uint64 {
	return d.hashes
}

// FindDuplicates возвращает список групп дубликатов и пути межзапусковых дубликатов.
// Алгоритм:
//  1. Группировка по размеру.
//  2. Для групп с ≥2 файлами вычисляется xxhash.
//  3. Группировка по хешу внутри размерной группы.
//  4. Пары Live Photos (.HEIC + .MOV с одним basename) исключаются из дубликатов.
//  5. Если хеш файла есть в knownHashes (из state) — файл считается межзапусковым дубликатом.
func (d *Deduper) FindDuplicates(ctx context.Context) ([]Result, []string, error) {
	if len(d.files) == 0 {
		return nil, nil, nil
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
	var crossRunDups []string

	// 2. Обрабатываем только группы с ≥2 файлами.
	for _, group := range sizeGroups {
		if len(group) < 2 {
			continue
		}

		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}

		// 3. Вычисляем хеш для каждого файла в группе.
		// Ошибка хеширования (например, файл удалён или стал недоступным)
		// не прерывает всю операцию — файл просто исключается из дедупликации.
		var hashed []fileHash
		for _, f := range group {
			h, err := hasher.HashFile(ctx, f.Path)
			if err != nil {
				continue
			}
			d.hashes[f.Path] = h
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

			// Межзапусковая дедупликация: если хеш группы есть в knownHashes,
			// все файлы в группе считаются дубликатами уже скопированного файла.
			if _, known := d.knownHashes[hashGroup[0].hash]; known {
				for _, fh := range hashGroup {
					crossRunDups = append(crossRunDups, fh.info.Path)
				}
				continue
			}

			infos := make([]scanner.FileInfo, len(hashGroup))
			for i, fh := range hashGroup {
				infos[i] = fh.info
			}

			original := PickOriginal(infos, d.strategy, d.dateSources)
			var duplicates []scanner.FileInfo

			for _, fh := range hashGroup {
				candidate := fh.info
				if candidate.Path == original.Path {
					continue
				}
				if d.livePhotos && isLivePhotoPair(original, candidate) {
					continue
				}
				if d.rawJPEGClustering && isRawJPEGPair(original, candidate) {
					continue
				}
				duplicates = append(duplicates, candidate)
			}

			if len(duplicates) > 0 {
				results = append(results, Result{
					Original:   original,
					Duplicates: duplicates,
					Hash:       hashGroup[0].hash,
				})
			}
		}
	}

	return results, crossRunDups, nil
}

// isRawJPEGPair возвращает true, если два файла являются парой RAW + JPEG:
// одинаковый basename (без расширения), один — RAW, другой — JPEG.
func isRawJPEGPair(a, b scanner.FileInfo) bool {
	baseA := strings.TrimSuffix(a.Name, filepath.Ext(a.Name))
	baseB := strings.TrimSuffix(b.Name, filepath.Ext(b.Name))
	if !strings.EqualFold(baseA, baseB) {
		return false
	}

	extA := strings.ToLower(a.Ext)
	extB := strings.ToLower(b.Ext)

	rawExts := map[string]bool{".cr2": true, ".nef": true, ".arw": true, ".dng": true, ".raf": true}
	jpegExts := map[string]bool{".jpg": true, ".jpeg": true}

	isRaw := rawExts[extA]
	isJpeg := jpegExts[extB]
	if isRaw && isJpeg {
		return true
	}

	isRaw = rawExts[extB]
	isJpeg = jpegExts[extA]
	if isRaw && isJpeg {
		return true
	}

	return false
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
