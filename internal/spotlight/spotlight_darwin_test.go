//go:build darwin

package spotlight

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
	"howett.net/plist"
)

func TestWriteTagsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.jpg")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	date := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	if err := WriteTags(f, date); err != nil {
		t.Fatalf("write tags: %v", err)
	}

	// Читаем kMDItemUserTags.
	size, err := unix.Getxattr(f, xattrUserTags, nil)
	if err != nil {
		t.Fatalf("getxattr size: %v", err)
	}
	buf := make([]byte, size)
	if _, err := unix.Getxattr(f, xattrUserTags, buf); err != nil {
		t.Fatalf("getxattr: %v", err)
	}

	var tags []string
	if _, err := plist.Unmarshal(buf, &tags); err != nil {
		t.Fatalf("unmarshal plist: %v", err)
	}
	if len(tags) != 1 || tags[0] != "2024-03-15" {
		t.Errorf("expected tags=[2024-03-15], got %v", tags)
	}

	// Читаем kMDItemComment.
	size, err = unix.Getxattr(f, xattrComment, nil)
	if err != nil {
		t.Fatalf("getxattr comment size: %v", err)
	}
	buf = make([]byte, size)
	if _, err := unix.Getxattr(f, xattrComment, buf); err != nil {
		t.Fatalf("getxattr comment: %v", err)
	}

	var comment string
	if _, err := plist.Unmarshal(buf, &comment); err != nil {
		t.Fatalf("unmarshal comment plist: %v", err)
	}
	if comment != "2024-03-15" {
		t.Errorf("expected comment=2024-03-15, got %q", comment)
	}
}

func TestWriteTags_ZeroDate(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.jpg")
	os.WriteFile(f, []byte("x"), 0644)

	if err := WriteTags(f, time.Time{}); err == nil {
		t.Error("expected error for zero date")
	}
}
