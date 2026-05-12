// Package collision предоставляет стратегии разрешения конфликтов имён файлов.
package collision

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cespare/xxhash/v2"
)

// Strategy определяет способ генерации суффикса при коллизии имён.
type Strategy string

const (
	// StrategyCounter добавляет числовой суффикс _1, _2, _3…
	StrategyCounter Strategy = "counter"
	// StrategyHash добавляет короткий hex-хеш от пути исходного файла.
	StrategyHash Strategy = "hash"
)

// Resolve возвращает кандидат имени файла для разрешения коллизии.
// target — изначально желаемый путь (уже занят).
// sourcePath — путь к исходному файлу (используется для хеш-стратегии).
// index — порядковый номер попытки (0-based).
//
//	Для counter: index=0 → _1, index=1 → _2.
//	Для hash:    index=0 → _<hash>, index=1 → _<hash>_1.
func Resolve(target string, strategy Strategy, sourcePath string, index int) string {
	dir := filepath.Dir(target)
	ext := filepath.Ext(target)
	base := strings.TrimSuffix(filepath.Base(target), ext)

	switch strategy {
	case StrategyHash:
		h := xxhash.Sum64String(sourcePath)
		shortHash := fmt.Sprintf("%06x", h&0xFFFFFF)
		if index == 0 {
			return filepath.Join(dir, base+"_"+shortHash+ext)
		}
		return filepath.Join(dir, base+"_"+shortHash+fmt.Sprintf("_%d", index)+ext)
	default:
		return filepath.Join(dir, base+fmt.Sprintf("_%d", index+1)+ext)
	}
}
