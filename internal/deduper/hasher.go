package deduper

import (
	"io"
	"os"

	"github.com/cespare/xxhash/v2"
)

// HashFile вычисляет xxhash для содержимого файла.
// Чтение потоковое — подходит для файлов любого размера.
func HashFile(path string) (uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	h := xxhash.New()
	buf := make([]byte, 64*1024)

	for {
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
