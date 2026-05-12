package deduper

import (
	"testing"
	"time"

	"photo-sorter/internal/dateresolver"
	"photo-sorter/internal/scanner"
)

func TestPickOriginal_Empty(t *testing.T) {
	var empty []scanner.FileInfo
	got := PickOriginal(empty, StrategyPath, nil)
	if got.Path != "" {
		t.Errorf("expected empty result for empty input, got %v", got)
	}
}

func TestPickOriginal_Single(t *testing.T) {
	files := []scanner.FileInfo{{Path: "/a.jpg", Name: "a.jpg", Size: 100}}
	got := PickOriginal(files, StrategyPath, nil)
	if got.Path != "/a.jpg" {
		t.Errorf("expected /a.jpg, got %s", got.Path)
	}
}

func TestPickOriginal_Path(t *testing.T) {
	files := []scanner.FileInfo{
		{Path: "/b.jpg", Name: "b.jpg", Size: 100},
		{Path: "/a.jpg", Name: "a.jpg", Size: 200},
	}
	got := PickOriginal(files, StrategyPath, nil)
	if got.Path != "/a.jpg" {
		t.Errorf("expected /a.jpg (alphabetical), got %s", got.Path)
	}
}

func TestPickOriginal_Largest(t *testing.T) {
	files := []scanner.FileInfo{
		{Path: "/a.jpg", Name: "a.jpg", Size: 100},
		{Path: "/b.jpg", Name: "b.jpg", Size: 200},
	}
	got := PickOriginal(files, StrategyLargest, nil)
	if got.Path != "/b.jpg" {
		t.Errorf("expected /b.jpg (largest), got %s", got.Path)
	}
}

func TestPickOriginal_Largest_TieFallbackToPath(t *testing.T) {
	files := []scanner.FileInfo{
		{Path: "/b.jpg", Name: "b.jpg", Size: 100},
		{Path: "/a.jpg", Name: "a.jpg", Size: 100},
	}
	got := PickOriginal(files, StrategyLargest, nil)
	if got.Path != "/a.jpg" {
		t.Errorf("expected /a.jpg (tie → path), got %s", got.Path)
	}
}

func TestPickOriginal_Newest(t *testing.T) {
	now := time.Now()
	files := []scanner.FileInfo{
		{Path: "/a.jpg", Name: "a.jpg", Size: 100, ModTime: now.Add(-time.Hour)},
		{Path: "/b.jpg", Name: "b.jpg", Size: 100, ModTime: now},
	}
	got := PickOriginal(files, StrategyNewest, nil)
	if got.Path != "/b.jpg" {
		t.Errorf("expected /b.jpg (newest), got %s", got.Path)
	}
}

func TestPickOriginal_Newest_TieFallbackToLargestThenPath(t *testing.T) {
	now := time.Now()
	files := []scanner.FileInfo{
		{Path: "/b.jpg", Name: "b.jpg", Size: 100, ModTime: now},
		{Path: "/a.jpg", Name: "a.jpg", Size: 200, ModTime: now},
	}
	got := PickOriginal(files, StrategyNewest, nil)
	if got.Path != "/a.jpg" {
		t.Errorf("expected /a.jpg (tie → largest), got %s", got.Path)
	}
}

func TestPickOriginal_Newest_TieLargest_TieFallbackToPath(t *testing.T) {
	now := time.Now()
	files := []scanner.FileInfo{
		{Path: "/b.jpg", Name: "b.jpg", Size: 100, ModTime: now},
		{Path: "/a.jpg", Name: "a.jpg", Size: 100, ModTime: now},
	}
	got := PickOriginal(files, StrategyNewest, nil)
	if got.Path != "/a.jpg" {
		t.Errorf("expected /a.jpg (tie → path), got %s", got.Path)
	}
}

func TestPickOriginal_BestMeta(t *testing.T) {
	files := []scanner.FileInfo{
		{Path: "/a.jpg", Name: "a.jpg", Size: 100},
		{Path: "/b.jpg", Name: "b.jpg", Size: 100},
		{Path: "/c.jpg", Name: "c.jpg", Size: 100},
	}
	sources := map[string]dateresolver.Source{
		"/a.jpg": dateresolver.SourceFilename,
		"/b.jpg": dateresolver.SourceExif,
		"/c.jpg": dateresolver.SourceNone,
	}
	got := PickOriginal(files, StrategyBestMeta, sources)
	if got.Path != "/b.jpg" {
		t.Errorf("expected /b.jpg (EXIF best), got %s", got.Path)
	}
}

func TestPickOriginal_BestMeta_NilFallbackToLargest(t *testing.T) {
	files := []scanner.FileInfo{
		{Path: "/a.jpg", Name: "a.jpg", Size: 100},
		{Path: "/b.jpg", Name: "b.jpg", Size: 200},
	}
	got := PickOriginal(files, StrategyBestMeta, nil)
	if got.Path != "/b.jpg" {
		t.Errorf("expected /b.jpg (nil sources → largest), got %s", got.Path)
	}
}

func TestPickOriginal_BestMeta_TieFallbackToLargestThenPath(t *testing.T) {
	files := []scanner.FileInfo{
		{Path: "/b.jpg", Name: "b.jpg", Size: 100},
		{Path: "/a.jpg", Name: "a.jpg", Size: 200},
	}
	sources := map[string]dateresolver.Source{
		"/a.jpg": dateresolver.SourceFilename,
		"/b.jpg": dateresolver.SourceFilename,
	}
	got := PickOriginal(files, StrategyBestMeta, sources)
	if got.Path != "/a.jpg" {
		t.Errorf("expected /a.jpg (tie source → largest), got %s", got.Path)
	}
}
