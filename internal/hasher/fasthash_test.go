package hasher

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cespare/xxhash/v2"
)

func TestFastHash_SmallFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small.bin")
	data := []byte("hello fasthash")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	hash, err := FastHash(path)
	if err != nil {
		t.Fatalf("FastHash: %v", err)
	}

	// Для файла ≤ 128KB результат должен совпадать с полным хешем.
	want := xxhash.Sum64(data)
	if hash != want {
		t.Errorf("hash mismatch: got %d, want %d", hash, want)
	}
}

func TestFastHash_LargeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.bin")

	// Создаём файл размером 256KB (больше 128KB).
	size := 256 * 1024
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 256)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	h1, err := FastHash(path)
	if err != nil {
		t.Fatalf("FastHash first: %v", err)
	}
	h2, err := FastHash(path)
	if err != nil {
		t.Fatalf("FastHash second: %v", err)
	}

	if h1 != h2 {
		t.Errorf("FastHash not stable: %d != %d", h1, h2)
	}

	// Результат НЕ должен совпадать с полным хешем.
	full := xxhash.Sum64(data)
	if h1 == full {
		t.Error("FastHash for large file unexpectedly equals full hash")
	}
}

func TestFastHash_DifferentContent(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.bin")
	pathB := filepath.Join(dir, "b.bin")

	// Одинаковый размер, разное содержимое.
	dataA := make([]byte, 200*1024)
	dataB := make([]byte, 200*1024)
	for i := range dataA {
		dataA[i] = byte(i % 7)
		dataB[i] = byte(i % 13)
	}
	if err := os.WriteFile(pathA, dataA, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.WriteFile(pathB, dataB, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	h1, err := FastHash(pathA)
	if err != nil {
		t.Fatalf("FastHash: %v", err)
	}
	h2, err := FastHash(pathB)
	if err != nil {
		t.Fatalf("FastHash: %v", err)
	}

	if h1 == h2 {
		t.Error("expected different hashes for different content")
	}
}

func TestFastHash_NotRegular(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "link")
	if err := os.Symlink("nonexistent", path); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	_, err := FastHash(path)
	if err == nil {
		t.Error("expected error for symlink")
	}
}
