package hasher

import (
	"fmt"
	"io"
	"os"

	"github.com/cespare/xxhash/v2"
)

const fastHashChunk = 64 * 1024 // 64KB

// FastHash вычисляет "быстрый" хеш файла: xxhash первых 64KB + последних 64KB.
// Для файлов ≤ 128KB считается хеш всего файла (эквивалентно HashFile).
// Используется для быстрой проверки "возможно дубликат / точно не дубликат"
// между запусками без чтения всего файла.
func FastHash(path string) (uint64, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, err
	}
	if !info.Mode().IsRegular() {
		return 0, fmt.Errorf("not a regular file: %s", path)
	}

	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	// TOCTOU-защита: проверяем, что открытый fd — всё ещё обычный файл.
	if stat, err := f.Stat(); err != nil || !stat.Mode().IsRegular() {
		if err != nil {
			return 0, err
		}
		return 0, fmt.Errorf("not a regular file: %s", path)
	}

	size := info.Size()
	h := xxhash.New()

	if size <= 2*fastHashChunk {
		// Файл маленький — хешируем целиком.
		if _, err := io.Copy(h, f); err != nil {
			return 0, err
		}
		return h.Sum64(), nil
	}

	// Первые 64KB.
	if _, err := io.CopyN(h, f, fastHashChunk); err != nil {
		return 0, err
	}

	// Последние 64KB.
	if _, err := f.Seek(-fastHashChunk, io.SeekEnd); err != nil {
		return 0, err
	}
	if _, err := io.CopyN(h, f, fastHashChunk); err != nil {
		return 0, err
	}

	return h.Sum64(), nil
}
