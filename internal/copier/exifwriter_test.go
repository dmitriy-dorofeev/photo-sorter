package copier

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteExifDate_NotFound(t *testing.T) {
	err := writeExifDate(context.Background(), "/nonexistent/exiftool", "dummy.jpg", time.Date(2024, 3, 15, 14, 30, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("expected error for missing exiftool")
	}
}

func TestWriteExifDate_ZeroDate(t *testing.T) {
	err := writeExifDate(context.Background(), "exiftool", "dummy.jpg", time.Time{})
	if err == nil {
		t.Fatal("expected error for zero date")
	}
}

func TestWriteExifDate_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping exiftool integration test in short mode")
	}

	if _, err := exec.LookPath("exiftool"); err != nil {
		t.Skip("exiftool not found in PATH")
	}

	// Копируем реальный JPEG из testdata
	srcPath := filepath.Join("..", "..", "testdata", "dateresolver", "minimal.jpg")
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jpg")
	data, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	os.WriteFile(path, data, 0644)

	date := time.Date(2024, 3, 15, 14, 30, 22, 0, time.UTC)
	err = writeExifDate(context.Background(), "exiftool", path, date)
	if err != nil {
		t.Fatalf("writeExifDate failed: %v", err)
	}

	// Читаем обратно
	cmd := exec.Command("exiftool", "-DateTimeOriginal", "-s3", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("exiftool read failed: %v (output: %s)", err, string(out))
	}
	got := string(out)
	want := "2024:03:15 14:30:22\n"
	if got != want {
		t.Errorf("DateTimeOriginal = %q, want %q", got, want)
	}
}
