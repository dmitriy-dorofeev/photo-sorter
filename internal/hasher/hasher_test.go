package hasher

import (
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/cespare/xxhash/v2"
)

func TestHashFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.bin")
	data := []byte("hello world")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	hash, err := HashFile(context.Background(), path)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}

	want := xxhash.Sum64String("hello world")
	if hash != want {
		t.Errorf("hash mismatch: got %d, want %d", hash, want)
	}
}

func TestHashFile_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.bin")
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	hash, err := HashFile(context.Background(), path)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}

	want := xxhash.Sum64String("")
	if hash != want {
		t.Errorf("hash mismatch: got %d, want %d", hash, want)
	}
}

func TestHashFile_NotRegular(t *testing.T) {
	dir := t.TempDir()

	// directory
	_, err := HashFile(context.Background(), dir)
	if err == nil {
		t.Error("expected error for directory, got nil")
	}

	// symlink
	target := filepath.Join(dir, "target.bin")
	if err := os.WriteFile(target, []byte("x"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	link := filepath.Join(dir, "link.bin")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	_, err = HashFile(context.Background(), link)
	if err == nil {
		t.Error("expected error for symlink, got nil")
	}
}

func BenchmarkHashFile(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "data.bin")
	data := make([]byte, 10*1024*1024) // 10 MB
	if _, err := rand.Read(data); err != nil {
		b.Fatalf("rand.Read: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		b.Fatalf("write file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := HashFile(context.Background(), path)
		if err != nil {
			b.Fatal(err)
		}
	}
}
