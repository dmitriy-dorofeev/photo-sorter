// Package hasher вычисляет хеши файлов.
package hasher

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/cespare/xxhash/v2"
)

// HashFile вычисляет xxhash для содержимого файла.
// Перед открытием проверяет, что файл — обычный (не FIFO, не symlink и т.д.).
// Чтение потоковое — подходит для файлов любого размера.
// Проверяет ctx.Err() между блоками чтения, позволяя прервать хеширование больших файлов.
func HashFile(ctx context.Context, path string) (uint64, error) {
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

	h := xxhash.New()
	buf := make([]byte, 64*1024)

	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		n, err := f.Read(buf)
		if n > 0 {
			_, _ = h.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
	}

	return h.Sum64(), nil
}
