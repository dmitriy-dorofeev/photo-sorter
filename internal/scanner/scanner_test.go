package scanner

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestScan_FilterExtensions(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.jpg"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("bbb"), 0644)
	os.WriteFile(filepath.Join(dir, "c.mov"), []byte("cc"), 0644)

	s := New([]string{dir}, ".jpg", ".mov")
	files, err := s.Scan()
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
	files, err := s.Scan()
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
	files, err := s.Scan()
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
	files, err := s.Scan()
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
}
