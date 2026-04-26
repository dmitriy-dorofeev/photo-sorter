package sorter

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"photo-sorter/internal/deduper"
	"photo-sorter/internal/scanner"
)

func date(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func TestBuildTree_Basic(t *testing.T) {
	s := New("/target", "2006/01/02", true)
	files := []scanner.FileInfo{
		{Path: "/src/a.jpg", Name: "a.jpg", Ext: ".jpg"},
	}

	resolve := func(f scanner.FileInfo) (time.Time, bool) {
		return date(2024, 3, 15), true
	}

	entries := s.BuildTree(context.Background(), files, nil, resolve)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	want := filepath.Join("/target", "2024", "03", "15", "a.jpg")
	if entries[0].Target != want {
		t.Errorf("target = %q, want %q", entries[0].Target, want)
	}
}

func TestBuildTree_Unsorted(t *testing.T) {
	s := New("/target", "2006/01/02", true)
	files := []scanner.FileInfo{
		{Path: "/src/unknown.bin", Name: "unknown.bin", Ext: ".bin"},
	}

	resolve := func(f scanner.FileInfo) (time.Time, bool) {
		return time.Time{}, false
	}

	entries := s.BuildTree(context.Background(), files, nil, resolve)
	want := filepath.Join("/target", "unsorted", "unknown.bin")
	if entries[0].Target != want {
		t.Errorf("target = %q, want %q", entries[0].Target, want)
	}
}

func TestBuildTree_SkipDuplicates(t *testing.T) {
	s := New("/target", "2006/01/02", true)
	files := []scanner.FileInfo{
		{Path: "/src/original.jpg", Name: "original.jpg"},
		{Path: "/src/dup.jpg", Name: "dup.jpg"},
	}

	resolve := func(f scanner.FileInfo) (time.Time, bool) {
		return date(2024, 1, 1), true
	}

	dups := []deduper.Result{
		{
			Original:   files[0],
			Duplicates: []scanner.FileInfo{files[1]},
		},
	}

	entries := s.BuildTree(context.Background(), files, dups, resolve)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Skip {
		t.Error("original should not be skipped")
	}
	if !entries[1].Skip {
		t.Error("duplicate should be skipped")
	}
}

func TestBuildTree_NameCollision(t *testing.T) {
	s := New("/target", "2006/01/02", true)
	files := []scanner.FileInfo{
		{Path: "/src1/a.jpg", Name: "a.jpg"},
		{Path: "/src2/a.jpg", Name: "a.jpg"},
	}

	resolve := func(f scanner.FileInfo) (time.Time, bool) {
		return date(2024, 1, 1), true
	}

	entries := s.BuildTree(context.Background(), files, nil, resolve)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Target == entries[1].Target {
		t.Error("targets should differ")
	}
	if entries[1].Target != filepath.Join("/target", "2024", "01", "01", "a_1.jpg") {
		t.Errorf("unexpected collision target: %q", entries[1].Target)
	}
}

func TestBuildTree_LivePhotos(t *testing.T) {
	s := New("/target", "2006/01/02", true)
	files := []scanner.FileInfo{
		{Path: "/src/IMG_1234.HEIC", Name: "IMG_1234.HEIC", Ext: ".heic"},
		{Path: "/src/IMG_1234.MOV", Name: "IMG_1234.MOV", Ext: ".mov"},
	}

	resolve := func(f scanner.FileInfo) (time.Time, bool) {
		if f.Ext == ".heic" {
			return date(2024, 5, 20), true
		}
		return time.Time{}, false // .MOV без даты
	}

	entries := s.BuildTree(context.Background(), files, nil, resolve)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	want := filepath.Join("/target", "2024", "05", "20")
	if filepath.Dir(entries[0].Target) != want {
		t.Errorf("HEIC target dir = %q, want %q", filepath.Dir(entries[0].Target), want)
	}
	if filepath.Dir(entries[1].Target) != want {
		t.Errorf("MOV target dir = %q, want %q", filepath.Dir(entries[1].Target), want)
	}
}

func TestBuildTree_LivePhotosDisabled(t *testing.T) {
	s := New("/target", "2006/01/02", false)
	files := []scanner.FileInfo{
		{Path: "/src/IMG_1234.HEIC", Name: "IMG_1234.HEIC", Ext: ".heic"},
		{Path: "/src/IMG_1234.MOV", Name: "IMG_1234.MOV", Ext: ".mov"},
	}

	resolve := func(f scanner.FileInfo) (time.Time, bool) {
		if f.Ext == ".heic" {
			return date(2024, 5, 20), true
		}
		return time.Time{}, false // .MOV без даты
	}

	entries := s.BuildTree(context.Background(), files, nil, resolve)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	want := filepath.Join("/target", "2024", "05", "20")
	if filepath.Dir(entries[0].Target) != want {
		t.Errorf("HEIC target dir = %q, want %q", filepath.Dir(entries[0].Target), want)
	}
	// .MOV should go to unsorted because livePhotos is disabled
	wantUnsorted := filepath.Join("/target", "unsorted", "IMG_1234.MOV")
	if entries[1].Target != wantUnsorted {
		t.Errorf("MOV target = %q, want %q", entries[1].Target, wantUnsorted)
	}
}
