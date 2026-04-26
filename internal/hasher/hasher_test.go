package hasher

import (
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
)

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
