package sorter

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"photo-sorter/internal/collision"
	"photo-sorter/internal/dateresolver"
	"photo-sorter/internal/deduper"
	"photo-sorter/internal/renamer"
	"photo-sorter/internal/scanner"
)

func date(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func TestBuildTree_Basic(t *testing.T) {
	s := New("/target", "2006/01/02", true, true, nil, collision.StrategyCounter)
	files := []scanner.FileInfo{
		{Path: "/src/a.jpg", Name: "a.jpg", Ext: ".jpg"},
	}

	resolve := func(_ context.Context, f scanner.FileInfo) (time.Time, bool) {
		return date(2024, 3, 15), true
	}

	entries, err := s.BuildTree(context.Background(), files, nil, resolve, nil)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	want := filepath.Join("/target", "2024", "03", "15", "a.jpg")
	if entries[0].Target != want {
		t.Errorf("target = %q, want %q", entries[0].Target, want)
	}
}

func TestBuildTree_Unsorted(t *testing.T) {
	s := New("/target", "2006/01/02", true, true, nil, collision.StrategyCounter)
	files := []scanner.FileInfo{
		{Path: "/src/unknown.bin", Name: "unknown.bin", Ext: ".bin"},
	}

	resolve := func(_ context.Context, f scanner.FileInfo) (time.Time, bool) {
		return time.Time{}, false
	}

	entries, err := s.BuildTree(context.Background(), files, nil, resolve, nil)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}
	want := filepath.Join("/target", UnsortedDir, "unknown.bin")
	if entries[0].Target != want {
		t.Errorf("target = %q, want %q", entries[0].Target, want)
	}
}

func TestBuildTree_SkipDuplicates(t *testing.T) {
	s := New("/target", "2006/01/02", true, true, nil, collision.StrategyCounter)
	files := []scanner.FileInfo{
		{Path: "/src/original.jpg", Name: "original.jpg"},
		{Path: "/src/dup.jpg", Name: "dup.jpg"},
	}

	resolve := func(_ context.Context, f scanner.FileInfo) (time.Time, bool) {
		return date(2024, 1, 1), true
	}

	dups := []deduper.Result{
		{
			Original:   files[0],
			Duplicates: []scanner.FileInfo{files[1]},
		},
	}

	entries, err := s.BuildTree(context.Background(), files, dups, resolve, nil)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}
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
	s := New("/target", "2006/01/02", true, true, nil, collision.StrategyCounter)
	files := []scanner.FileInfo{
		{Path: "/src1/a.jpg", Name: "a.jpg"},
		{Path: "/src2/a.jpg", Name: "a.jpg"},
	}

	resolve := func(_ context.Context, f scanner.FileInfo) (time.Time, bool) {
		return date(2024, 1, 1), true
	}

	entries, err := s.BuildTree(context.Background(), files, nil, resolve, nil)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}
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

func buildLivePhotoEntries(t *testing.T, livePhotos bool) []Entry {
	s := New("/target", "2006/01/02", livePhotos, true, nil, collision.StrategyCounter)
	files := []scanner.FileInfo{
		{Path: "/src/IMG_1234.HEIC", Name: "IMG_1234.HEIC", Ext: ".heic"},
		{Path: "/src/IMG_1234.MOV", Name: "IMG_1234.MOV", Ext: ".mov"},
	}

	resolve := func(_ context.Context, f scanner.FileInfo) (time.Time, bool) {
		if f.Ext == ".heic" {
			return date(2024, 5, 20), true
		}
		return time.Time{}, false // .MOV без даты
	}

	entries, err := s.BuildTree(context.Background(), files, nil, resolve, nil)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	return entries
}

func TestBuildTree_LivePhotos(t *testing.T) {
	entries := buildLivePhotoEntries(t, true)
	want := filepath.Join("/target", "2024", "05", "20")
	if filepath.Dir(entries[0].Target) != want {
		t.Errorf("HEIC target dir = %q, want %q", filepath.Dir(entries[0].Target), want)
	}
	if filepath.Dir(entries[1].Target) != want {
		t.Errorf("MOV target dir = %q, want %q", filepath.Dir(entries[1].Target), want)
	}
}

func TestBuildTree_LivePhotosDisabled(t *testing.T) {
	entries := buildLivePhotoEntries(t, false)
	want := filepath.Join("/target", "2024", "05", "20")
	if filepath.Dir(entries[0].Target) != want {
		t.Errorf("HEIC target dir = %q, want %q", filepath.Dir(entries[0].Target), want)
	}
	// .MOV should go to unsorted because livePhotos is disabled
	wantUnsorted := filepath.Join("/target", UnsortedDir, "IMG_1234.MOV")
	if entries[1].Target != wantUnsorted {
		t.Errorf("MOV target = %q, want %q", entries[1].Target, wantUnsorted)
	}
}

func TestBuildTree_WithFileNameTemplate(t *testing.T) {
	tmpl, _ := renamer.Parse("{YYYY}-{MM}-{DD}_{original}{ext}")
	s := New("/target", "2006/01/02", true, true, tmpl, collision.StrategyCounter)
	files := []scanner.FileInfo{
		{Path: "/src/a.jpg", Name: "a.jpg", Ext: ".jpg"},
	}

	resolve := func(_ context.Context, f scanner.FileInfo) (time.Time, bool) {
		return date(2024, 3, 15), true
	}

	entries, err := s.BuildTree(context.Background(), files, nil, resolve, nil)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}
	want := filepath.Join("/target", "2024", "03", "15", "2024-03-15_a.jpg")
	if entries[0].Target != want {
		t.Errorf("target = %q, want %q", entries[0].Target, want)
	}
}

func TestBuildTree_TemplatePreservesExtension(t *testing.T) {
	tmpl, _ := renamer.Parse("{original}{ext}")
	s := New("/target", "2006/01/02", true, true, tmpl, collision.StrategyCounter)
	files := []scanner.FileInfo{
		{Path: "/src/photo.JPG", Name: "photo.JPG", Ext: ".jpg"},
	}

	resolve := func(_ context.Context, f scanner.FileInfo) (time.Time, bool) {
		return date(2024, 3, 15), true
	}

	entries, err := s.BuildTree(context.Background(), files, nil, resolve, nil)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}
	want := filepath.Join("/target", "2024", "03", "15", "photo.JPG")
	if entries[0].Target != want {
		t.Errorf("target = %q, want %q", entries[0].Target, want)
	}
}

func TestBuildTree_CollisionWithTemplate(t *testing.T) {
	tmpl, _ := renamer.Parse("{YYYY}{MM}{DD}{ext}")
	s := New("/target", "2006/01/02", true, true, tmpl, collision.StrategyCounter)
	files := []scanner.FileInfo{
		{Path: "/src1/a.jpg", Name: "a.jpg", Ext: ".jpg"},
		{Path: "/src2/b.jpg", Name: "b.jpg", Ext: ".jpg"},
	}

	resolve := func(_ context.Context, f scanner.FileInfo) (time.Time, bool) {
		return date(2024, 1, 1), true
	}

	entries, err := s.BuildTree(context.Background(), files, nil, resolve, nil)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	want1 := filepath.Join("/target", "2024", "01", "01", "20240101.jpg")
	want2 := filepath.Join("/target", "2024", "01", "01", "20240101_1.jpg")
	if entries[0].Target != want1 {
		t.Errorf("entry[0] target = %q, want %q", entries[0].Target, want1)
	}
	if entries[1].Target != want2 {
		t.Errorf("entry[1] target = %q, want %q", entries[1].Target, want2)
	}
}

func TestBuildTree_CollisionTemplateSeq(t *testing.T) {
	tmpl, _ := renamer.Parse("{seq}{ext}")
	s := New("/target", "2006/01/02", true, true, tmpl, collision.StrategyCounter)
	files := []scanner.FileInfo{
		{Path: "/src1/a.jpg", Name: "a.jpg", Ext: ".jpg"},
		{Path: "/src2/b.jpg", Name: "b.jpg", Ext: ".jpg"},
		{Path: "/src3/c.jpg", Name: "c.jpg", Ext: ".jpg"},
	}

	resolve := func(_ context.Context, f scanner.FileInfo) (time.Time, bool) {
		return date(2024, 1, 1), true
	}

	entries, err := s.BuildTree(context.Background(), files, nil, resolve, nil)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	want := []string{
		filepath.Join("/target", "2024", "01", "01", "1.jpg"),
		filepath.Join("/target", "2024", "01", "01", "2.jpg"),
		filepath.Join("/target", "2024", "01", "01", "3.jpg"),
	}
	for i, w := range want {
		if entries[i].Target != w {
			t.Errorf("entry[%d] target = %q, want %q", i, entries[i].Target, w)
		}
	}
}

func TestBuildTree_UnsortedWithTemplate(t *testing.T) {
	tmpl, _ := renamer.Parse("{YYYY}-{MM}-{DD}_{original}{ext}")
	s := New("/target", "2006/01/02", true, true, tmpl, collision.StrategyCounter)
	files := []scanner.FileInfo{
		{Path: "/src/unknown.jpg", Name: "unknown.jpg", Ext: ".jpg"},
	}

	resolve := func(_ context.Context, f scanner.FileInfo) (time.Time, bool) {
		return time.Time{}, false
	}

	entries, err := s.BuildTree(context.Background(), files, nil, resolve, nil)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}
	want := filepath.Join("/target", UnsortedDir, "0000-00-00_unknown.jpg")
	if entries[0].Target != want {
		t.Errorf("target = %q, want %q", entries[0].Target, want)
	}
}

func TestBuildTree_LivePhotosDevicePropagation(t *testing.T) {
	tmpl, _ := renamer.Parse("{device}_{original}{ext}")
	s := New("/target", "2006/01/02", true, true, tmpl, collision.StrategyCounter)
	files := []scanner.FileInfo{
		{Path: "/src/photo.HEIC", Name: "photo.HEIC", Ext: ".heic", Device: "iPhone"},
		{Path: "/src/photo.MOV", Name: "photo.MOV", Ext: ".mov", Device: "iPhone"},
	}

	resolve := func(_ context.Context, f scanner.FileInfo) (time.Time, bool) {
		if f.Ext == ".heic" {
			return date(2024, 5, 20), true
		}
		return time.Time{}, false
	}

	entries, err := s.BuildTree(context.Background(), files, nil, resolve, nil)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	want0 := filepath.Join("/target", "2024", "05", "20", "iPhone_photo.HEIC")
	want1 := filepath.Join("/target", "2024", "05", "20", "iPhone_photo.MOV")
	if entries[0].Target != want0 {
		t.Errorf("HEIC target = %q, want %q", entries[0].Target, want0)
	}
	if entries[1].Target != want1 {
		t.Errorf("MOV target = %q, want %q", entries[1].Target, want1)
	}
}

func TestBuildTree_DeviceInTemplate(t *testing.T) {
	tmpl, _ := renamer.Parse("{device}/{YYYY}/{original}{ext}")
	s := New("/target", "2006/01/02", true, true, tmpl, collision.StrategyCounter)
	files := []scanner.FileInfo{
		{Path: "/src/PXL_123.jpg", Name: "PXL_123.jpg", Ext: ".jpg", Device: "Pixel"},
	}

	resolve := func(_ context.Context, f scanner.FileInfo) (time.Time, bool) {
		return date(2024, 3, 15), true
	}

	entries, err := s.BuildTree(context.Background(), files, nil, resolve, nil)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}
	want := filepath.Join("/target", "2024", "03", "15", "Pixel", "2024", "PXL_123.jpg")
	if entries[0].Target != want {
		t.Errorf("target = %q, want %q", entries[0].Target, want)
	}
}

func TestBuildTree_NameCollisionHash(t *testing.T) {
	s := New("/target", "2006/01/02", true, true, nil, collision.StrategyHash)
	files := []scanner.FileInfo{
		{Path: "/src1/a.jpg", Name: "a.jpg"},
		{Path: "/src2/a.jpg", Name: "a.jpg"},
	}

	resolve := func(_ context.Context, f scanner.FileInfo) (time.Time, bool) {
		return date(2024, 1, 1), true
	}

	entries, err := s.BuildTree(context.Background(), files, nil, resolve, nil)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Target == entries[1].Target {
		t.Error("targets should differ")
	}
	// Первый файл остаётся без суффикса, второй получает hash-суффикс
	wantFirst := filepath.Join("/target", "2024", "01", "01", "a.jpg")
	if entries[0].Target != wantFirst {
		t.Errorf("first target = %q, want %q", entries[0].Target, wantFirst)
	}
	if !strings.Contains(entries[1].Target, "_") {
		t.Errorf("second target should contain suffix: %q", entries[1].Target)
	}
}

func TestBuildTree_NameCollisionHashFallback(t *testing.T) {
	// Искусственно создаём ситуацию, когда хеш даёт одинаковый суффикс.
	// Так как xxhash от разных путей практически не коллизирует,
	// проверяем логику fallback через targetCounts: добавляем три файла
	// с одинаковым именем и проверяем, что все получают уникальные имена.
	s := New("/target", "2006/01/02", true, true, nil, collision.StrategyHash)
	files := []scanner.FileInfo{
		{Path: "/src1/a.jpg", Name: "a.jpg"},
		{Path: "/src2/a.jpg", Name: "a.jpg"},
		{Path: "/src3/a.jpg", Name: "a.jpg"},
	}

	resolve := func(_ context.Context, f scanner.FileInfo) (time.Time, bool) {
		return date(2024, 1, 1), true
	}

	entries, err := s.BuildTree(context.Background(), files, nil, resolve, nil)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	seen := make(map[string]struct{})
	for _, e := range entries {
		if _, ok := seen[e.Target]; ok {
			t.Fatalf("duplicate target: %q", e.Target)
		}
		seen[e.Target] = struct{}{}
	}
}

func TestBuildTree_DateSource(t *testing.T) {
	s := New("/target", "2006/01/02", true, true, nil, collision.StrategyCounter)
	files := []scanner.FileInfo{
		{Path: "/src/a.jpg", Name: "a.jpg", Ext: ".jpg"},
		{Path: "/src/b.jpg", Name: "b.jpg", Ext: ".jpg"},
	}

	resolve := func(_ context.Context, f scanner.FileInfo) (time.Time, bool) {
		return date(2024, 3, 15), true
	}

	dateSources := map[string]dateresolver.Source{
		"/src/a.jpg": dateresolver.SourceExif,
		"/src/b.jpg": dateresolver.SourceFilename,
	}

	entries, err := s.BuildTree(context.Background(), files, nil, resolve, dateSources)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].DateSource != dateresolver.SourceExif {
		t.Errorf("entry[0] DateSource = %v, want SourceExif", entries[0].DateSource)
	}
	if entries[1].DateSource != dateresolver.SourceFilename {
		t.Errorf("entry[1] DateSource = %v, want SourceFilename", entries[1].DateSource)
	}
}

func TestBuildTree_LivePhotos_DateSource(t *testing.T) {
	s := New("/target", "2006/01/02", true, true, nil, collision.StrategyCounter)
	files := []scanner.FileInfo{
		{Path: "/src/IMG_1234.HEIC", Name: "IMG_1234.HEIC", Ext: ".heic"},
		{Path: "/src/IMG_1234.MOV", Name: "IMG_1234.MOV", Ext: ".mov"},
	}

	resolve := func(_ context.Context, f scanner.FileInfo) (time.Time, bool) {
		if f.Ext == ".heic" {
			return date(2024, 5, 20), true
		}
		return time.Time{}, false
	}

	dateSources := map[string]dateresolver.Source{
		"/src/IMG_1234.HEIC": dateresolver.SourceExif,
		"/src/IMG_1234.MOV":  dateresolver.SourceNone,
	}

	entries, err := s.BuildTree(context.Background(), files, nil, resolve, dateSources)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].DateSource != dateresolver.SourceExif {
		t.Errorf("HEIC DateSource = %v, want SourceExif", entries[0].DateSource)
	}
	if entries[1].DateSource != dateresolver.SourceExif {
		t.Errorf("MOV DateSource = %v, want SourceExif (inherited from HEIC)", entries[1].DateSource)
	}
}

func TestBuildTree_DateSource_None(t *testing.T) {
	s := New("/target", "2006/01/02", true, true, nil, collision.StrategyCounter)
	files := []scanner.FileInfo{
		{Path: "/src/unknown.bin", Name: "unknown.bin", Ext: ".bin"},
	}

	resolve := func(_ context.Context, f scanner.FileInfo) (time.Time, bool) {
		return time.Time{}, false
	}

	dateSources := map[string]dateresolver.Source{
		"/src/unknown.bin": dateresolver.SourceNone,
	}

	entries, err := s.BuildTree(context.Background(), files, nil, resolve, dateSources)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].DateSource != dateresolver.SourceNone {
		t.Errorf("DateSource = %v, want SourceNone", entries[0].DateSource)
	}
}

func TestBuildTree_RawJPEGClustering(t *testing.T) {
	s := New("/target", "2006/01/02", true, true, nil, collision.StrategyCounter)
	files := []scanner.FileInfo{
		{Path: "/src/IMG_0001.JPG", Name: "IMG_0001.JPG", Ext: ".jpg"},
		{Path: "/src/IMG_0001.CR2", Name: "IMG_0001.CR2", Ext: ".cr2"},
	}

	resolve := func(_ context.Context, f scanner.FileInfo) (time.Time, bool) {
		if f.Ext == ".jpg" {
			return date(2024, 6, 10), true
		}
		return time.Time{}, false
	}

	entries, err := s.BuildTree(context.Background(), files, nil, resolve, nil)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	want := filepath.Join("/target", "2024", "06", "10")
	if filepath.Dir(entries[0].Target) != want {
		t.Errorf("JPG target dir = %q, want %q", filepath.Dir(entries[0].Target), want)
	}
	if filepath.Dir(entries[1].Target) != want {
		t.Errorf("CR2 target dir = %q, want %q", filepath.Dir(entries[1].Target), want)
	}
}

func TestBuildTree_RawJPEGClusteringDisabled(t *testing.T) {
	s := New("/target", "2006/01/02", true, false, nil, collision.StrategyCounter)
	files := []scanner.FileInfo{
		{Path: "/src/IMG_0001.JPG", Name: "IMG_0001.JPG", Ext: ".jpg"},
		{Path: "/src/IMG_0001.CR2", Name: "IMG_0001.CR2", Ext: ".cr2"},
	}

	resolve := func(_ context.Context, f scanner.FileInfo) (time.Time, bool) {
		if f.Ext == ".jpg" {
			return date(2024, 6, 10), true
		}
		return time.Time{}, false
	}

	entries, err := s.BuildTree(context.Background(), files, nil, resolve, nil)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	wantJPG := filepath.Join("/target", "2024", "06", "10")
	if filepath.Dir(entries[0].Target) != wantJPG {
		t.Errorf("JPG target dir = %q, want %q", filepath.Dir(entries[0].Target), wantJPG)
	}
	wantCR2 := filepath.Join("/target", UnsortedDir, "IMG_0001.CR2")
	if entries[1].Target != wantCR2 {
		t.Errorf("CR2 target = %q, want %q", entries[1].Target, wantCR2)
	}
}
