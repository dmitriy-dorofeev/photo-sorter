package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestScan_FilterExtensions(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.jpg"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("bbb"), 0644)
	os.WriteFile(filepath.Join(dir, "c.mov"), []byte("cc"), 0644)

	s := New([]string{dir}, ".jpg", ".mov")
	files, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	names := []string{files[0].Name, files[1].Name}
	sort.Strings(names)
	if names[0] != "a.jpg" || names[1] != "c.mov" {
		t.Errorf("unexpected names: %v", names)
	}
}

func TestScan_NoFilter(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "x.png"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(dir, "y"), []byte("yy"), 0644)

	s := New([]string{dir})
	files, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
}

func TestScan_ParallelSources(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	os.WriteFile(filepath.Join(dir1, "a.jpg"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir2, "b.jpg"), []byte("bb"), 0644)

	s := New([]string{dir1, dir2}, ".jpg")
	files, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	names := []string{files[0].Name, files[1].Name}
	sort.Strings(names)
	if names[0] != "a.jpg" || names[1] != "b.jpg" {
		t.Errorf("unexpected names: %v", names)
	}
}

func TestScan_Subdirs(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "root.jpg"), []byte("r"), 0644)
	os.WriteFile(filepath.Join(dir, "sub", "nested.jpg"), []byte("n"), 0644)

	s := New([]string{dir}, ".jpg")
	files, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
}

func TestScan_NonExistentSource(t *testing.T) {
	s := New([]string{"/nonexistent/path/12345"})
	_, err := s.Scan(context.Background())
	if err == nil {
		t.Error("expected error for nonexistent source")
	}
}

func TestScan_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	s := New([]string{dir}, ".jpg")
	files, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestScan_PermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "accessible"), 0755)
	os.WriteFile(filepath.Join(dir, "accessible", "a.jpg"), []byte("a"), 0644)

	locked := filepath.Join(dir, "locked")
	os.MkdirAll(locked, 0755)
	os.WriteFile(filepath.Join(locked, "b.jpg"), []byte("b"), 0644)
	if err := os.Chmod(locked, 0000); err != nil {
		t.Skipf("cannot chmod: %v", err)
	}
	defer os.Chmod(locked, 0755)

	s := New([]string{dir}, ".jpg")
	files, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file (accessible only), got %d", len(files))
	}
	if files[0].Name != "a.jpg" {
		t.Errorf("expected a.jpg, got %s", files[0].Name)
	}

	skipped := s.SkippedPaths()
	if len(skipped) == 0 {
		t.Error("expected skipped paths to contain locked files")
	}
	found := false
	for _, p := range skipped {
		if strings.Contains(p, "locked") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected skipped paths to contain locked dir, got %v", skipped)
	}
}

func TestScan_Symlink(t *testing.T) {
	dir := t.TempDir()
	realFile := filepath.Join(dir, "real.jpg")
	os.WriteFile(realFile, []byte("x"), 0644)
	linkFile := filepath.Join(dir, "link.jpg")
	if err := os.Symlink(realFile, linkFile); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	s := New([]string{dir}, ".jpg")
	files, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	// Non-regular файлы (symlink, FIFO, device) пропускаются
	// во избежание abort pipeline в deduper/hasher.
	if len(files) != 1 {
		t.Fatalf("expected 1 file (real only, symlink skipped), got %d", len(files))
	}
	if files[0].Name != "real.jpg" {
		t.Errorf("expected real.jpg, got %s", files[0].Name)
	}
}

func TestScan_ContextCancel(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	// dir1: много файлов
	for i := 0; i < 50; i++ {
		os.WriteFile(filepath.Join(dir1, "file"+string(rune('a'+i%26))+".jpg"), []byte("x"), 0644)
	}
	// dir2: один файл
	os.WriteFile(filepath.Join(dir2, "single.jpg"), []byte("y"), 0644)

	ctx, cancel := context.WithCancel(context.Background())
	s := New([]string{dir1, dir2}, ".jpg")

	// Отменяем сразу — один из воркеров должен поймать ctx.Done().
	cancel()

	_, err := s.Scan(ctx)
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestScan_ManyFiles(t *testing.T) {
	dir := t.TempDir()
	want := 500
	for i := 0; i < want; i++ {
		name := filepath.Join(dir, fmt.Sprintf("file%03d.jpg", i))
		os.WriteFile(name, []byte("x"), 0644)
	}

	s := New([]string{dir}, ".jpg")
	start := time.Now()
	files, err := s.Scan(context.Background())
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(files) != want {
		t.Fatalf("expected %d files, got %d", want, len(files))
	}
	t.Logf("scanned %d files in %v", want, elapsed)
}
