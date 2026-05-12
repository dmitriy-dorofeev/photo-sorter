// Стратегии выбора оригинала из группы дубликатов.
package deduper

import (
	"sort"

	"photo-sorter/internal/dateresolver"
	"photo-sorter/internal/scanner"
)

// Strategy определяет, какой файл считать оригиналом в группе дубликатов.
type Strategy string

const (
	StrategyPath     Strategy = "path"
	StrategyLargest  Strategy = "largest"
	StrategyNewest   Strategy = "newest"
	StrategyBestMeta Strategy = "best-meta"
)

// PickOriginal выбирает лучший файл из группы дубликатов по заданной стратегии.
// При равенстве основного критерия используется fallback для детерминированности:
//
//	largest → path, newest → largest → path, best-meta → largest → path.
func PickOriginal(
	files []scanner.FileInfo,
	strategy Strategy,
	dateSources map[string]dateresolver.Source,
) scanner.FileInfo {
	if len(files) == 0 {
		return scanner.FileInfo{}
	}
	if len(files) == 1 {
		return files[0]
	}

	switch strategy {
	case StrategyLargest:
		return pickByLargest(files)
	case StrategyNewest:
		return pickByNewest(files)
	case StrategyBestMeta:
		return pickByBestMeta(files, dateSources)
	default:
		return pickByPath(files)
	}
}

// pickByPath — детерминированный выбор по алфавиту пути (текущее поведение).
func pickByPath(files []scanner.FileInfo) scanner.FileInfo {
	best := files[0]
	for _, f := range files[1:] {
		if f.Path < best.Path {
			best = f
		}
	}
	return best
}

// pickByLargest — выбирает файл с максимальным размером.
// При равенстве размеров fallback на pickByPath.
func pickByLargest(files []scanner.FileInfo) scanner.FileInfo {
	best := files[0]
	for _, f := range files[1:] {
		if f.Size > best.Size {
			best = f
		} else if f.Size == best.Size && f.Path < best.Path {
			best = f
		}
	}
	return best
}

// pickByNewest — выбирает файл с самым поздним ModTime.
// При равенстве времени fallback на largest, затем на path.
func pickByNewest(files []scanner.FileInfo) scanner.FileInfo {
	best := files[0]
	for _, f := range files[1:] {
		if f.ModTime.After(best.ModTime) {
			best = f
		} else if f.ModTime.Equal(best.ModTime) {
			if f.Size > best.Size {
				best = f
			} else if f.Size == best.Size && f.Path < best.Path {
				best = f
			}
		}
	}
	return best
}

// pickByBestMeta — выбирает файл с наиболее надёжным источником даты.
// Если dateSources не передана, fallback на largest.
// При равенстве Source fallback на largest, затем на path.
func pickByBestMeta(files []scanner.FileInfo, dateSources map[string]dateresolver.Source) scanner.FileInfo {
	if len(dateSources) == 0 {
		return pickByLargest(files)
	}

	best := files[0]
	bestSrc := sourceFor(best.Path, dateSources)
	for _, f := range files[1:] {
		src := sourceFor(f.Path, dateSources)
		if src > bestSrc {
			best = f
			bestSrc = src
		} else if src == bestSrc {
			if f.Size > best.Size {
				best = f
			} else if f.Size == best.Size && f.Path < best.Path {
				best = f
			}
		}
	}
	return best
}

func sourceFor(path string, dateSources map[string]dateresolver.Source) dateresolver.Source {
	if s, ok := dateSources[path]; ok {
		return s
	}
	return dateresolver.SourceNone
}

// sortedByPath возвращает копию слайса, отсортированную по пути.
// Используется в тестах и при необходимости детерминированного обхода.
func sortedByPath(files []scanner.FileInfo) []scanner.FileInfo {
	out := make([]scanner.FileInfo, len(files))
	copy(out, files)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})
	return out
}
