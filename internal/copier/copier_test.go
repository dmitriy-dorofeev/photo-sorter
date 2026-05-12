package copier

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"photo-sorter/internal/collision"
	"photo-sorter/internal/hasher"
	"photo-sorter/internal/scanner"
	"photo-sorter/internal/sorter"
)

func TestCopy_DryRun(t *testing.T) {
	dir := t.TempDir()
	c := New(true, dir, collision.StrategyCounter)

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

	c := New(false, dstDir, collision.StrategyCounter)
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

	c := New(false, dstDir, collision.StrategyCounter)
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

	c := New(false, dstDir, collision.StrategyCounter)
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

	c := New(false, dstDir, collision.StrategyCounter)
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

	c := New(false, dstDir, collision.StrategyCounter)
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

func TestCopy_ErrorList(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	c := New(false, dstDir, collision.StrategyCounter)
	// Три записи с несуществующими исходными файлами.
	entries := []sorter.Entry{
		{Source: scanner.FileInfo{Path: filepath.Join(srcDir, "missing1.jpg"), Name: "missing1.jpg", Size: 1}, Target: filepath.Join(dstDir, "missing1.jpg")},
		{Source: scanner.FileInfo{Path: filepath.Join(srcDir, "missing2.jpg"), Name: "missing2.jpg", Size: 1}, Target: filepath.Join(dstDir, "missing2.jpg")},
		{Source: scanner.FileInfo{Path: filepath.Join(srcDir, "missing3.jpg"), Name: "missing3.jpg", Size: 1}, Target: filepath.Join(dstDir, "missing3.jpg")},
	}

	stats, err := c.Copy(context.Background(), entries, nil)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if stats.Errors != 3 {
		t.Errorf("expected 3 errors, got %d", stats.Errors)
	}
	if len(stats.ErrorList) != 3 {
		t.Errorf("expected 3 error entries, got %d", len(stats.ErrorList))
	}
}

func TestCopy_AbortOnMissingTarget(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := filepath.Join(t.TempDir(), "target")
	os.MkdirAll(dstDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "a.jpg"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(srcDir, "b.jpg"), []byte("y"), 0644)
	os.WriteFile(filepath.Join(srcDir, "c.jpg"), []byte("z"), 0644)

	c := New(false, dstDir, collision.StrategyCounter)
	entries := []sorter.Entry{
		{Source: scanner.FileInfo{Path: filepath.Join(srcDir, "a.jpg"), Name: "a.jpg", Size: 1}, Target: filepath.Join(dstDir, "a.jpg")},
		{Source: scanner.FileInfo{Path: filepath.Join(srcDir, "b.jpg"), Name: "b.jpg", Size: 1}, Target: filepath.Join(dstDir, "b.jpg")},
		{Source: scanner.FileInfo{Path: filepath.Join(srcDir, "c.jpg"), Name: "c.jpg", Size: 1}, Target: filepath.Join(dstDir, "c.jpg")},
	}

	// Превращаем целевую директорию в обычный файл, чтобы os.MkdirAll падал.
	os.RemoveAll(dstDir)
	os.WriteFile(dstDir, []byte("not a dir"), 0644)

	_, err := c.Copy(context.Background(), entries, nil)
	if err == nil {
		t.Fatal("expected error when target is not a directory")
	}
	want := "too many consecutive target errors (3), aborting"
	if err.Error() != want {
		t.Fatalf("unexpected error: %v (want %s)", err, want)
	}
}

func TestCopy_PathTraversal(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "a.jpg"), []byte("x"), 0644)

	c := New(false, dstDir, collision.StrategyCounter)
	entries := []sorter.Entry{
		{Source: scanner.FileInfo{Path: filepath.Join(srcDir, "a.jpg"), Name: "a.jpg", Size: 1}, Target: filepath.Join(dstDir, "..", "outside.jpg")},
	}

	stats, err := c.Copy(context.Background(), entries, nil)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if stats.Errors != 1 {
		t.Errorf("expected 1 error, got %d", stats.Errors)
	}
	if len(stats.ErrorList) != 1 {
		t.Errorf("expected 1 error entry, got %d", len(stats.ErrorList))
	}
	if _, e := os.Stat(filepath.Join(dstDir, "..", "outside.jpg")); !os.IsNotExist(e) {
		t.Error("path traversal should not create file outside target")
	}
}

func TestCopy_SymlinkAttack(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "a.jpg")
	os.WriteFile(srcFile, []byte("new content"), 0644)

	// Создаём файл-жертву за пределами target.
	victim := filepath.Join(t.TempDir(), "victim.txt")
	os.WriteFile(victim, []byte("victim"), 0644)

	// Создаём symlink в target, указывающий на victim.
	symlink := filepath.Join(dstDir, "a.jpg")
	if err := os.Symlink(victim, symlink); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	c := New(false, dstDir, collision.StrategyCounter)
	entries := []sorter.Entry{
		{Source: scanner.FileInfo{Path: srcFile, Name: "a.jpg", Size: 11}, Target: symlink},
	}

	stats, err := c.Copy(context.Background(), entries, nil)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if stats.Copied != 1 {
		t.Errorf("expected 1 copied, got %d", stats.Copied)
	}

	// Проверяем, что symlink заменён обычным файлом.
	info, err := os.Lstat(symlink)
	if err != nil {
		t.Fatalf("lstat failed: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("symlink was not replaced with regular file")
	}
	data, err := os.ReadFile(symlink)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(data) != "new content" {
		t.Errorf("content mismatch: %q", string(data))
	}

	// Проверяем, что victim не был перезаписан.
	victimData, err := os.ReadFile(victim)
	if err != nil {
		t.Fatalf("read victim: %v", err)
	}
	if string(victimData) != "victim" {
		t.Error("symlink attack succeeded: victim was overwritten")
	}
}

func TestCopy_NotEnoughDiskSpace(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "a.jpg")
	os.WriteFile(srcFile, []byte("hello"), 0644)

	c := &Copier{dryRun: false, targetRoot: dstDir, collisionStrategy: collision.StrategyCounter, spaceFunc: func(string) (uint64, error) {
		return 1, nil // 1 байт свободно, нужно 5
	}, hashFunc: hasher.HashFile}
	entries := []sorter.Entry{
		{Source: scanner.FileInfo{Path: srcFile, Name: "a.jpg", Size: 5}, Target: filepath.Join(dstDir, "a.jpg")},
	}

	_, err := c.Copy(context.Background(), entries, nil)
	if err == nil {
		t.Fatal("expected error for not enough disk space")
	}
	if !os.IsNotExist(err) {
		// Должна быть ошибка "not enough disk space"
		if err.Error() != "not enough disk space: need 5 bytes, have 1 bytes" {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestCopy_ContextCancelMidway(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	for i := 0; i < 3; i++ {
		os.WriteFile(filepath.Join(srcDir, fmt.Sprintf("f%d.txt", i)), []byte("x"), 0644)
	}

	c := New(false, dstDir, collision.StrategyCounter)
	var entries []sorter.Entry
	for i := 0; i < 3; i++ {
		entries = append(entries, sorter.Entry{
			Source: scanner.FileInfo{Path: filepath.Join(srcDir, fmt.Sprintf("f%d.txt", i)), Name: fmt.Sprintf("f%d.txt", i), Size: 1},
			Target: filepath.Join(dstDir, fmt.Sprintf("f%d.txt", i)),
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Отменяем после обработки первого файла.
	var cancelOnce sync.Once
	_, err := c.Copy(ctx, entries, func(cur, tot int) {
		if cur >= 1 {
			cancelOnce.Do(cancel)
		}
	})
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestCopy_IntegrityCheck_Success(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "a.jpg")
	os.WriteFile(srcFile, []byte("hello"), 0644)

	c := New(false, dstDir, collision.StrategyCounter)
	entries := []sorter.Entry{
		{Source: scanner.FileInfo{Path: srcFile, Name: "a.jpg", Size: 5}, Target: filepath.Join(dstDir, "a.jpg")},
	}

	stats, err := c.Copy(context.Background(), entries, nil)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if stats.Copied != 1 {
		t.Errorf("expected 1 copied, got %d", stats.Copied)
	}
	if stats.IntegrityFailures != 0 {
		t.Errorf("expected 0 integrity failures, got %d", stats.IntegrityFailures)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "a.jpg")); err != nil {
		t.Errorf("expected file to exist: %v", err)
	}
}

func TestCopy_IntegrityCheck_Failure(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "a.jpg")
	os.WriteFile(srcFile, []byte("hello"), 0644)

	c := &Copier{
		dryRun:            false,
		targetRoot:        dstDir,
		collisionStrategy: collision.StrategyCounter,
		spaceFunc:         availableSpace,
		hashFunc: func(ctx context.Context, path string) (uint64, error) {
			if strings.Contains(path, dstDir) {
				return 0xBAD, nil
			}
			return 0xCAFE, nil
		},
	}
	entries := []sorter.Entry{
		{Source: scanner.FileInfo{Path: srcFile, Name: "a.jpg", Size: 5}, Target: filepath.Join(dstDir, "a.jpg")},
	}

	stats, err := c.Copy(context.Background(), entries, nil)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if stats.Errors != 1 {
		t.Errorf("expected 1 error, got %d", stats.Errors)
	}
	if stats.IntegrityFailures != 1 {
		t.Errorf("expected 1 integrity failure, got %d", stats.IntegrityFailures)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "a.jpg")); !os.IsNotExist(err) {
		t.Errorf("expected corrupted file to be removed")
	}
}

func TestCopy_IntegrityCheck_HashError(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "a.jpg")
	os.WriteFile(srcFile, []byte("hello"), 0644)

	c := &Copier{
		dryRun:            false,
		targetRoot:        dstDir,
		collisionStrategy: collision.StrategyCounter,
		spaceFunc:         availableSpace,
		hashFunc: func(ctx context.Context, path string) (uint64, error) {
			if strings.Contains(path, dstDir) {
				return 0, errors.New("disk io error")
			}
			return hasher.HashFile(ctx, path)
		},
	}
	entries := []sorter.Entry{
		{Source: scanner.FileInfo{Path: srcFile, Name: "a.jpg", Size: 5}, Target: filepath.Join(dstDir, "a.jpg")},
	}

	stats, err := c.Copy(context.Background(), entries, nil)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if stats.Errors != 1 {
		t.Errorf("expected 1 error, got %d", stats.Errors)
	}
	if stats.IntegrityFailures != 1 {
		t.Errorf("expected 1 integrity failure, got %d", stats.IntegrityFailures)
	}
	if _, e := os.Stat(filepath.Join(dstDir, "a.jpg")); !os.IsNotExist(e) {
		t.Errorf("expected file to be removed after hash error")
	}
}

func TestCopy_IntegrityCheck_DryRun(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "a.jpg")
	os.WriteFile(srcFile, []byte("hello"), 0644)

	c := &Copier{
		dryRun:            true,
		targetRoot:        dstDir,
		collisionStrategy: collision.StrategyCounter,
		spaceFunc:         availableSpace,
		hashFunc: func(ctx context.Context, path string) (uint64, error) {
			return 0, errors.New("should not be called")
		},
	}
	entries := []sorter.Entry{
		{Source: scanner.FileInfo{Path: srcFile, Name: "a.jpg", Size: 5}, Target: filepath.Join(dstDir, "a.jpg")},
	}

	stats, err := c.Copy(context.Background(), entries, nil)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if stats.Copied != 1 {
		t.Errorf("expected 1 copied (dry-run), got %d", stats.Copied)
	}
	if stats.IntegrityFailures != 0 {
		t.Errorf("expected 0 integrity failures in dry-run, got %d", stats.IntegrityFailures)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "a.jpg")); !os.IsNotExist(err) {
		t.Error("dry-run should not create files")
	}
}

func TestCopy_CollisionHashOnDisk(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "a.jpg")
	os.WriteFile(srcFile, []byte("new content"), 0644)
	os.WriteFile(filepath.Join(dstDir, "a.jpg"), []byte("old content"), 0644)

	c := New(false, dstDir, collision.StrategyHash)
	entries := []sorter.Entry{
		{Source: scanner.FileInfo{Path: srcFile, Name: "a.jpg", Size: 11}, Target: filepath.Join(dstDir, "a.jpg")},
	}

	stats, err := c.Copy(context.Background(), entries, nil)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if stats.Copied != 1 {
		t.Errorf("expected 1 copied, got %d", stats.Copied)
	}
	// Проверяем, что имя изменилось на hash-суффикс
	if entries[0].Target == filepath.Join(dstDir, "a.jpg") {
		t.Error("Entry.Target should be updated after hash collision resolution")
	}
	if _, err := os.Stat(entries[0].Target); err != nil {
		t.Errorf("expected file to exist at %s: %v", entries[0].Target, err)
	}
}

func TestCopy_CollisionHashSameContent(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "a.jpg")
	os.WriteFile(srcFile, []byte("same"), 0644)
	os.WriteFile(filepath.Join(dstDir, "a.jpg"), []byte("same"), 0644)

	c := New(false, dstDir, collision.StrategyHash)
	entries := []sorter.Entry{
		{Source: scanner.FileInfo{Path: srcFile, Name: "a.jpg", Size: 4}, Target: filepath.Join(dstDir, "a.jpg")},
	}

	stats, err := c.Copy(context.Background(), entries, nil)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if stats.Skipped != 1 {
		t.Errorf("expected 1 skipped (same content), got %d", stats.Skipped)
	}
	// Entry.Target не должен меняться, т.к. файл не копировался
	if entries[0].Target != filepath.Join(dstDir, "a.jpg") {
		t.Errorf("Entry.Target should stay original when skipped: got %q", entries[0].Target)
	}
}

func TestCopy_CollisionCounterUpdatesTarget(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "a.jpg")
	os.WriteFile(srcFile, []byte("new"), 0644)
	os.WriteFile(filepath.Join(dstDir, "a.jpg"), []byte("old"), 0644)

	c := New(false, dstDir, collision.StrategyCounter)
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
	want := filepath.Join(dstDir, "a_1.jpg")
	if entries[0].Target != want {
		t.Errorf("Entry.Target = %q, want %q", entries[0].Target, want)
	}
}
