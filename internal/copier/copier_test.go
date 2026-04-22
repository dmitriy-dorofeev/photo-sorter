package copier

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"photo-sorter/internal/scanner"
	"photo-sorter/internal/sorter"
)

func TestCopy_DryRun(t *testing.T) {
	dir := t.TempDir()
	c := New(true, dir)

	entries := []sorter.Entry{
		{Source: scanner.FileInfo{Path: "/src/a.jpg", Name: "a.jpg"}, Target: filepath.Join(dir, "a.jpg")},
	}

	stats, err := c.Copy(context.Background(), entries, nil)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if stats.Copied != 1 {
		t.Errorf("expected 1 copied (dry-run), got %d", stats.Copied)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.jpg")); !os.IsNotExist(err) {
		t.Error("dry-run should not create files")
	}
}

func TestCopy_Basic(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "a.jpg")
	os.WriteFile(srcFile, []byte("hello"), 0644)

	c := New(false, dstDir)
	entries := []sorter.Entry{
		{Source: scanner.FileInfo{Path: srcFile, Name: "a.jpg", Size: 5}, Target: filepath.Join(dstDir, "2024", "a.jpg")},
	}

	stats, err := c.Copy(context.Background(), entries, nil)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if stats.Copied != 1 {
		t.Errorf("expected 1 copied, got %d", stats.Copied)
	}
	if stats.BytesCopied != 5 {
		t.Errorf("expected 5 bytes, got %d", stats.BytesCopied)
	}
	data, err := os.ReadFile(filepath.Join(dstDir, "2024", "a.jpg"))
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("content mismatch: %q", string(data))
	}
}

func TestCopy_SkipDuplicate(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "a.jpg")
	os.WriteFile(srcFile, []byte("dup"), 0644)
	os.WriteFile(filepath.Join(dstDir, "a.jpg"), []byte("dup"), 0644)

	c := New(false, dstDir)
	entries := []sorter.Entry{
		{Source: scanner.FileInfo{Path: srcFile, Name: "a.jpg", Size: 3}, Target: filepath.Join(dstDir, "a.jpg")},
	}

	stats, err := c.Copy(context.Background(), entries, nil)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if stats.Skipped != 1 {
		t.Errorf("expected 1 skipped (duplicate), got %d", stats.Skipped)
	}
}

func TestCopy_CollisionDifferent(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "a.jpg")
	os.WriteFile(srcFile, []byte("new"), 0644)
	os.WriteFile(filepath.Join(dstDir, "a.jpg"), []byte("old"), 0644)

	c := New(false, dstDir)
	entries := []sorter.Entry{
		{Source: scanner.FileInfo{Path: srcFile, Name: "a.jpg", Size: 3}, Target: filepath.Join(dstDir, "a.jpg")},
	}

	stats, err := c.Copy(context.Background(), entries, nil)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if stats.Copied != 1 {
		t.Errorf("expected 1 copied, got %d", stats.Copied)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "a_1.jpg")); err != nil {
		t.Errorf("expected a_1.jpg to exist: %v", err)
	}
}

func TestCopy_ContextCancel(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	for i := 0; i < 5; i++ {
		os.WriteFile(filepath.Join(srcDir, fmt.Sprintf("f%d.txt", i)), []byte("x"), 0644)
	}

	c := New(false, dstDir)
	var entries []sorter.Entry
	for i := 0; i < 5; i++ {
		entries = append(entries, sorter.Entry{
			Source: scanner.FileInfo{Path: filepath.Join(srcDir, fmt.Sprintf("f%d.txt", i)), Name: fmt.Sprintf("f%d.txt", i), Size: 1},
			Target: filepath.Join(dstDir, fmt.Sprintf("f%d.txt", i)),
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(10 * time.Millisecond) // даём контексту истечь

	_, err := c.Copy(ctx, entries, nil)
	if err == nil {
		t.Fatal("expected context error")
	}
}

func TestCopy_Progress(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "a.jpg"), []byte("x"), 0644)

	c := New(false, dstDir)
	entries := []sorter.Entry{
		{Source: scanner.FileInfo{Path: filepath.Join(srcDir, "a.jpg"), Name: "a.jpg", Size: 1}, Target: filepath.Join(dstDir, "a.jpg")},
	}

	var calls []int
	_, err := c.Copy(context.Background(), entries, func(cur, tot int) {
		calls = append(calls, cur)
	})
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if len(calls) == 0 {
		t.Error("progress not called")
	}
	if calls[len(calls)-1] != 1 {
		t.Errorf("last progress = %d, want 1", calls[len(calls)-1])
	}
}
