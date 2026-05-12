package internal

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"photo-sorter/internal/collision"
	"photo-sorter/internal/copier"
	"photo-sorter/internal/dateresolver"
	"photo-sorter/internal/deduper"
	"photo-sorter/internal/hasher"
	"photo-sorter/internal/runner"
	"photo-sorter/internal/scanner"
	"photo-sorter/internal/sorter"
)

func buildTreeAndCountUnsorted(t *testing.T, targetDir string, files []scanner.FileInfo, resolve func(context.Context, scanner.FileInfo) (time.Time, bool)) ([]sorter.Entry, int) {
	ded := deduper.New(files, true, deduper.StrategyPath, nil)
	dupResults, err := ded.FindDuplicates(context.Background())
	if err != nil {
		t.Fatalf("dedup failed: %v", err)
	}
	sort := sorter.New(targetDir, "2006/01/02", true, nil, collision.StrategyCounter)
	entries := sort.BuildTree(context.Background(), files, dupResults, resolve)

	unsortedCount := 0
	for _, e := range entries {
		if !e.Skip && sorter.IsUnsorted(e.Target) {
			unsortedCount++
		}
	}
	return entries, unsortedCount
}

func countFilesWithExts(dir string, exts ...string) int {
	want := make(map[string]struct{})
	for _, e := range exts {
		want[strings.ToLower(e)] = struct{}{}
	}
	var n int
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if len(want) > 0 {
			ext := strings.ToLower(filepath.Ext(d.Name()))
			if _, ok := want[ext]; !ok {
				return nil
			}
		}
		n++
		return nil
	})
	return n
}

func TestEndToEnd(t *testing.T) {
	sourceDir := filepath.Join("..", "testdata", "e2e", "source", "2023")
	targetDir := t.TempDir()

	// 1. Scan (include .png for filename-date test)
	sc := scanner.New([]string{sourceDir}, ".jpg", ".jpeg", ".heic", ".heif", ".mov", ".mp4", ".png")
	files, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	expectedCount := countFilesWithExts(sourceDir, ".jpg", ".jpeg", ".heic", ".heif", ".mov", ".mp4", ".png")
	if len(files) != expectedCount {
		t.Fatalf("expected %d files, got %d", expectedCount, len(files))
	}

	// 2. Resolve dates (UseModTime = true by default; force false for this test)
	resolver := dateresolver.New()
	resolver.UseModTime = false
	dateMap := make(map[string]time.Time)
	okMap := make(map[string]bool)
	for _, f := range files {
		d, ok := resolver.Resolve(context.Background(), f)
		dateMap[f.Path] = d
		okMap[f.Path] = ok
	}

	// Verify key dates
	for _, f := range files {
		name := filepath.Base(f.Path)
		switch name {
		case "DSC_0012.jpg":
			if !okMap[f.Path] || dateMap[f.Path].Year() != 2024 || dateMap[f.Path].Month() != time.March {
				t.Errorf("DSC_0012.jpg: expected EXIF date 2024-03, got %v (ok=%v)", dateMap[f.Path], okMap[f.Path])
			}
		case "IMG_20230415_143022.jpg":
			if !okMap[f.Path] || dateMap[f.Path].Format("2006-01-02") != "2024-03-15" {
				t.Errorf("IMG_20230415: expected EXIF 2024-03-15, got %v", dateMap[f.Path])
			}
		case "IMG_20230520_120000.jpg":
			if !okMap[f.Path] || dateMap[f.Path].Format("2006-01-02") != "2024-03-15" {
				t.Errorf("IMG_20230520: expected EXIF 2024-03-15, got %v", dateMap[f.Path])
			}
		case "IMG_20230710_120000.png":
			if !okMap[f.Path] || dateMap[f.Path].Format("2006-01-02") != "2023-07-10" {
				t.Errorf("IMG_20230710.png: expected 2023-07-10, got %v", dateMap[f.Path])
			}
		case "IMG_20230801_090000.jpg":
			if !okMap[f.Path] || dateMap[f.Path].Format("2006-01-02") != "2023-08-01" {
				t.Errorf("IMG_20230801.jpg: expected 2023-08-01, got %v", dateMap[f.Path])
			}
		case "photo_no_date.jpg":
			if okMap[f.Path] {
				t.Errorf("photo_no_date.jpg: expected no date with UseModTime=false, got %v", dateMap[f.Path])
			}
		case "live_photo.MOV":
			// With UseModTime=false, MOV without metadata should NOT resolve
			if okMap[f.Path] {
				t.Errorf("live_photo.MOV: expected no date with UseModTime=false, got %v", dateMap[f.Path])
			}
		case "live_photo.HEIC":
			// HEIC also has no EXIF support and no filename date
			if okMap[f.Path] {
				t.Errorf("live_photo.HEIC: expected no date with UseModTime=false, got %v", dateMap[f.Path])
			}
		case "video.mp4":
			if okMap[f.Path] {
				t.Errorf("video.mp4: expected no date with UseModTime=false, got %v", dateMap[f.Path])
			}
		}
	}

	// 3. Find duplicates
	ded := deduper.New(files, true, deduper.StrategyPath, nil)
	dupResults, err := ded.FindDuplicates(context.Background())
	if err != nil {
		t.Fatalf("dedup failed: %v", err)
	}

	// All minimal.jpg copies (6 files, 506 bytes) should form one duplicate group
	var minimalDupGroup *deduper.Result
	for i, r := range dupResults {
		if r.Original.Size == 506 && len(r.Duplicates) >= 5 {
			minimalDupGroup = &dupResults[i]
			break
		}
	}
	if minimalDupGroup == nil {
		t.Logf("dupResults: %+v", dupResults)
		t.Fatalf("expected duplicate group for minimal.jpg copies")
	}

	// Verify live_photo files are NOT marked as duplicates of each other
	for _, r := range dupResults {
		for _, d := range r.Duplicates {
			if strings.Contains(d.Name, "live_photo") {
				t.Errorf("live_photo file marked as duplicate: %s", d.Name)
			}
		}
		if strings.Contains(r.Original.Name, "live_photo") {
			t.Errorf("live_photo file marked as original duplicate: %s", r.Original.Name)
		}
	}

	// 4. Build tree
	sort := sorter.New(targetDir, "2006/01/02", true, nil, collision.StrategyCounter)
	entries := sort.BuildTree(context.Background(), files, dupResults, resolver.Resolve)
	if len(entries) != len(files) {
		t.Fatalf("expected %d entries, got %d", len(files), len(entries))
	}

	// Count skips and verify targets
	skipCount := 0
	unsortedCount := 0
	var copiedEntries []sorter.Entry
	for _, e := range entries {
		if e.Skip {
			skipCount++
		} else {
			copiedEntries = append(copiedEntries, e)
			if sorter.IsUnsorted(e.Target) {
				unsortedCount++
			}
		}
	}
	if skipCount != len(minimalDupGroup.Duplicates) {
		t.Errorf("expected %d skipped, got %d", len(minimalDupGroup.Duplicates), skipCount)
	}
	// Files without date: live_photo.HEIC, live_photo.MOV, video.mp4, photo_no_date.jpg = 4 unsorted
	if unsortedCount != 4 {
		t.Errorf("expected 3 unsorted entries, got %d", unsortedCount)
	}

	// 5. Dry-run copy
	c := copier.New(true, targetDir, collision.StrategyCounter)
	stats, err := c.Copy(context.Background(), entries, nil)
	if err != nil {
		t.Fatalf("dry-run copy failed: %v", err)
	}
	if stats.Copied != len(copiedEntries) {
		t.Errorf("dry-run: expected %d copied, got %d", len(copiedEntries), stats.Copied)
	}

	// Verify dry-run did NOT create files
	entriesInTarget, _ := os.ReadDir(targetDir)
	if len(entriesInTarget) > 0 {
		t.Errorf("dry-run created files in target: %d", len(entriesInTarget))
	}

	// 6. Real copy
	c2 := copier.New(false, targetDir, collision.StrategyCounter)
	stats2, err := c2.Copy(context.Background(), entries, nil)
	if err != nil {
		t.Fatalf("real copy failed: %v", err)
	}
	if stats2.Copied != len(copiedEntries) {
		t.Errorf("real copy: expected %d copied, got %d", len(copiedEntries), stats2.Copied)
	}
	if stats2.Errors > 0 {
		t.Errorf("real copy had %d errors", stats2.Errors)
	}
	if stats2.IntegrityFailures > 0 {
		t.Errorf("real copy had %d integrity failures", stats2.IntegrityFailures)
	}

	// Verify files exist and hashes match (integrity check)
	for _, e := range copiedEntries {
		if _, err := os.Stat(e.Target); os.IsNotExist(err) {
			t.Errorf("expected file to exist: %s", e.Target)
			continue
		}
		hSrc, err1 := hasher.HashFile(context.Background(), e.Source.Path)
		hDst, err2 := hasher.HashFile(context.Background(), e.Target)
		if err1 != nil || err2 != nil {
			t.Errorf("hash error for %s: srcErr=%v dstErr=%v", e.Source.Path, err1, err2)
			continue
		}
		if hSrc != hDst {
			t.Errorf("integrity mismatch for %s: srcHash=%x dstHash=%x", e.Source.Path, hSrc, hDst)
		}
	}

	// 7. Re-run copy (should skip all as duplicates)
	stats3, err := c2.Copy(context.Background(), entries, nil)
	if err != nil {
		t.Fatalf("re-copy failed: %v", err)
	}
	if stats3.Copied != 0 {
		t.Errorf("re-copy: expected 0 copied, got %d", stats3.Copied)
	}
	expectedSkipped := len(entries)
	if stats3.Skipped != expectedSkipped {
		t.Errorf("re-copy: expected %d skipped, got %d", expectedSkipped, stats3.Skipped)
	}
}

func TestEndToEnd_UseModTime(t *testing.T) {
	sourceDir := filepath.Join("..", "testdata", "e2e", "source", "2023")
	targetDir := t.TempDir()

	sc := scanner.New([]string{sourceDir}, ".jpg", ".jpeg", ".heic", ".heif", ".mov", ".mp4", ".png")
	files, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	// UseModTime = true: files without EXIF/filename should get ModTime
	resolver := dateresolver.New()
	resolver.UseModTime = true

	okMap := make(map[string]bool)
	for _, f := range files {
		_, ok := resolver.Resolve(context.Background(), f)
		okMap[f.Path] = ok
	}

	// live_photo.MOV should now resolve via ModTime
	if !okMap[filepath.Join(sourceDir, "live_photo.MOV")] {
		t.Errorf("live_photo.MOV: expected ModTime fallback when UseModTime=true")
	}
	// video.mp4 should also resolve via ModTime
	if !okMap[filepath.Join(sourceDir, "video.mp4")] {
		t.Errorf("video.mp4: expected ModTime fallback when UseModTime=true")
	}
	// live_photo.HEIC also has no EXIF/filename date, should get ModTime
	if !okMap[filepath.Join(sourceDir, "live_photo.HEIC")] {
		t.Errorf("live_photo.HEIC: expected ModTime fallback when UseModTime=true")
	}

	// Build tree: nothing should go to unsorted
	entries, unsortedCount := buildTreeAndCountUnsorted(t, targetDir, files, resolver.Resolve)
	if unsortedCount != 0 {
		t.Errorf("expected 0 unsorted with UseModTime=true, got %d", unsortedCount)
	}

	// Copy and verify
	c := copier.New(false, targetDir, collision.StrategyCounter)
	stats, err := c.Copy(context.Background(), entries, nil)
	if err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	if stats.Errors > 0 {
		t.Errorf("copy had %d errors", stats.Errors)
	}
}

func TestEndToEnd_ExtendedPatterns(t *testing.T) {
	sourceDir := filepath.Join("..", "testdata", "e2e", "source", "2024")
	targetDir := t.TempDir()

	sc := scanner.New([]string{sourceDir}, ".jpg", ".jpeg", ".png", ".mp4")
	files, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	expectedCount := countFilesWithExts(sourceDir, ".jpg", ".jpeg", ".png", ".mp4")
	if len(files) != expectedCount {
		t.Fatalf("expected %d files in 2024/, got %d", expectedCount, len(files))
	}

	resolver := dateresolver.New()
	resolver.UseModTime = false
	dateMap := make(map[string]time.Time)
	okMap := make(map[string]bool)
	for _, f := range files {
		d, ok := resolver.Resolve(context.Background(), f)
		dateMap[f.Path] = d
		okMap[f.Path] = ok
	}

	expectedDates := map[string]string{
		"IMG_20240115_143022.jpg":               "2024-01-15",
		"VID_20240120_120000.mp4":               "2024-01-20",
		"Screenshot 2024-02-10 at 12.30.45.png": "2024-02-10",
		"signal-2024-04-01-14-30-22.jpg":        "2024-04-01",
		"IMG-20240315-WA0001.jpg":               "2024-03-15",
		"PXL_20240501_143022123.jpg":            "2024-05-01",
		"20240315_143022.jpg":                   "2024-03-15",
		"2024-03-15 14.30.22.jpg":               "2024-03-15",
	}

	for name, wantDate := range expectedDates {
		found := false
		for _, f := range files {
			if filepath.Base(f.Path) == name {
				found = true
				if !okMap[f.Path] {
					t.Errorf("%s: expected date, got none", name)
					continue
				}
				got := dateMap[f.Path].Format("2006-01-02")
				if got != wantDate {
					t.Errorf("%s: expected %s, got %s", name, wantDate, got)
				}
			}
		}
		if !found {
			t.Errorf("%s: file not found in scan results", name)
		}
	}

	// DSC_1234.png has no date pattern and no EXIF
	for _, f := range files {
		if filepath.Base(f.Path) == "DSC_1234.png" {
			if okMap[f.Path] {
				t.Errorf("DSC_1234.png: expected no date, got %v", dateMap[f.Path])
			}
		}
	}

	// Build tree and verify targets
	entries, unsortedCount := buildTreeAndCountUnsorted(t, targetDir, files, resolver.Resolve)
	if unsortedCount != 1 {
		t.Errorf("expected 1 unsorted (DSC_1234.png), got %d", unsortedCount)
	}

	// Copy and verify
	c := copier.New(false, targetDir, collision.StrategyCounter)
	stats, err := c.Copy(context.Background(), entries, nil)
	if err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	if stats.Errors > 0 {
		t.Errorf("copy had %d errors", stats.Errors)
	}
}

func TestEndToEnd_VideoExifTool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping exiftool integration test in short mode")
	}

	srcDir := t.TempDir()
	targetDir := t.TempDir()

	// Create a fake video file with a parseable name to verify exiftool beats filename.
	movPath := filepath.Join(srcDir, "VID_20240120_120000.mov")
	os.WriteFile(movPath, []byte("fake video"), 0644)

	// Create fake exiftool that returns a different date.
	script := `#!/bin/sh
echo '[{"SourceFile":"'$3'","CreateDate":"2023:07:07 07:07:07"}]'
`
	fakeExifTool := filepath.Join(t.TempDir(), "fake_exiftool.sh")
	os.WriteFile(fakeExifTool, []byte(script), 0755)

	sc := scanner.New([]string{srcDir}, ".mov")
	files, err := sc.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	resolver := dateresolver.New()
	resolver.ExifToolPath = fakeExifTool

	d, ok := resolver.Resolve(context.Background(), files[0])
	if !ok {
		t.Fatal("expected date from exiftool")
	}
	want := time.Date(2023, 7, 7, 7, 7, 7, 0, time.UTC)
	if !d.Equal(want) {
		t.Fatalf("expected %v, got %v (exiftool should beat filename)", want, d)
	}

	// Verify tree uses exiftool date
	ded := deduper.New(files, true, deduper.StrategyPath, nil)
	dupResults, err := ded.FindDuplicates(context.Background())
	if err != nil {
		t.Fatalf("dedup failed: %v", err)
	}
	sort := sorter.New(targetDir, "2006/01/02", true, nil, collision.StrategyCounter)
	entries := sort.BuildTree(context.Background(), files, dupResults, resolver.Resolve)

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if sorter.IsUnsorted(entries[0].Target) {
		t.Errorf("expected dated target, got unsorted: %s", entries[0].Target)
	}
	if !strings.Contains(entries[0].Target, "2023/07/07") {
		t.Errorf("expected target in 2023/07/07, got %s", entries[0].Target)
	}

	c := copier.New(false, targetDir, collision.StrategyCounter)
	stats, err := c.Copy(context.Background(), entries, nil)
	if err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	if stats.Errors > 0 {
		t.Errorf("copy had %d errors", stats.Errors)
	}
}

func TestCancellation(t *testing.T) {
	sourceDir := filepath.Join("..", "testdata", "e2e", "source", "2023")
	targetDir := t.TempDir()

	sc := scanner.New([]string{sourceDir}, ".jpg", ".jpeg", ".heic", ".heif", ".mov", ".mp4", ".png")
	files, _ := sc.Scan(context.Background())
	resolver := dateresolver.New()
	ded := deduper.New(files, true, deduper.StrategyPath, nil)
	dupResults, err := ded.FindDuplicates(context.Background())
	if err != nil {
		t.Fatalf("dedup failed: %v", err)
	}
	sort := sorter.New(targetDir, "2006/01/02", true, nil, collision.StrategyCounter)
	entries := sort.BuildTree(context.Background(), files, dupResults, resolver.Resolve)

	ctx, cancel := context.WithCancel(context.Background())
	c := copier.New(false, targetDir, collision.StrategyCounter)

	// Cancel immediately
	cancel()
	stats, err := c.Copy(ctx, entries, nil)
	copied := stats.Copied
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if copied > 0 {
		t.Logf("cancelled after %d copies", copied)
	}
}

func TestEndToEnd_FileNameTemplate(t *testing.T) {
	sourceDir := filepath.Join("..", "testdata", "e2e", "source", "2023")
	targetDir := t.TempDir()

	cfg := runner.Config{
		Sources:          []string{sourceDir},
		Target:           targetDir,
		Template:         "2006/01/02",
		FileNameTemplate: "{YYYY}-{MM}-{DD}_{device}_{original}{ext}",
		LivePhotos:       true,
		IncludeVideo:     true,
		UseMTime:         false,
	}

	res, err := runner.Run(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Проверяем, что имена сгенерированы с устройством
	var foundDeviceName bool
	for _, e := range res.Entries {
		if !e.Skip && !sorter.IsUnsorted(e.Target) {
			base := filepath.Base(e.Target)
			if strings.Contains(base, "iPhone") || strings.Contains(base, "Camera") {
				foundDeviceName = true
			}
		}
	}
	if !foundDeviceName {
		t.Error("expected at least one file with device in generated name")
	}

	// Копируем
	c := copier.New(false, targetDir, collision.StrategyCounter)
	stats, err := c.Copy(context.Background(), res.Entries, nil)
	if err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	if stats.Errors > 0 {
		t.Errorf("copy had %d errors", stats.Errors)
	}
}

func TestEndToEnd_FileNameTemplate_Collision(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	// Два файла с одинаковой датой в имени, разное содержимое
	os.WriteFile(filepath.Join(sourceDir, "20240315_120000_a.jpg"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(sourceDir, "20240315_120000_b.jpg"), []byte("b"), 0644)

	cfg := runner.Config{
		Sources:          []string{sourceDir},
		Target:           targetDir,
		Template:         "2006/01/02",
		FileNameTemplate: "{YYYY}{MM}{DD}{ext}",
		LivePhotos:       false,
		IncludeVideo:     false,
		UseMTime:         false,
	}

	res, err := runner.Run(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(res.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(res.Entries))
	}

	// Один из файлов должен получить _1 из-за коллизии
	foundCollision := false
	for _, e := range res.Entries {
		if strings.Contains(filepath.Base(e.Target), "_1") {
			foundCollision = true
		}
	}
	if !foundCollision {
		t.Errorf("expected collision suffix _1, got targets: %v, %v", res.Entries[0].Target, res.Entries[1].Target)
	}
}

func TestEndToEnd_FileNameTemplate_Unsorted(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	// Файл без даты
	os.WriteFile(filepath.Join(sourceDir, "photo_no_date.jpg"), []byte("x"), 0644)

	cfg := runner.Config{
		Sources:          []string{sourceDir},
		Target:           targetDir,
		Template:         "2006/01/02",
		FileNameTemplate: "{YYYY}-{MM}-{DD}_{original}{ext}",
		LivePhotos:       false,
		IncludeVideo:     false,
		UseMTime:         false,
	}

	res, err := runner.Run(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(res.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(res.Entries))
	}

	if !sorter.IsUnsorted(res.Entries[0].Target) {
		t.Errorf("expected unsorted target, got %s", res.Entries[0].Target)
	}

	// Имя должно содержать 0000-00-00
	base := filepath.Base(res.Entries[0].Target)
	if !strings.HasPrefix(base, "0000-00-00_") {
		t.Errorf("expected zero-date prefix, got %s", base)
	}
}

func TestEndToEnd_CollisionStrategyHash(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	// Два файла с одинаковым именем, разное содержимое
	os.WriteFile(filepath.Join(sourceDir, "photo.jpg"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(sourceDir, "subdir", "photo.jpg"), []byte("b"), 0644)
	os.MkdirAll(filepath.Join(sourceDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(sourceDir, "subdir", "photo.jpg"), []byte("b"), 0644)

	cfg := runner.Config{
		Sources:           []string{sourceDir},
		Target:            targetDir,
		Template:          "2006/01/02",
		FileNameTemplate:  "{original}{ext}",
		LivePhotos:        false,
		IncludeVideo:      false,
		UseMTime:          true,
		CollisionStrategy: "hash",
	}

	res, err := runner.Run(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(res.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(res.Entries))
	}

	// Проверяем, что цели различаются и содержат hash-суффикс
	if res.Entries[0].Target == res.Entries[1].Target {
		t.Error("expected different targets for hash strategy")
	}

	c := copier.New(false, targetDir, collision.StrategyHash)
	stats, err := c.Copy(context.Background(), res.Entries, nil)
	if err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	if stats.Errors > 0 {
		t.Errorf("copy had %d errors", stats.Errors)
	}
	if stats.Copied != 2 {
		t.Errorf("expected 2 copied, got %d", stats.Copied)
	}

	// Проверяем, что Entry.Target обновлён в соответствии с реальными путями
	for _, e := range res.Entries {
		if _, err := os.Stat(e.Target); os.IsNotExist(err) {
			t.Errorf("file should exist at %s", e.Target)
		}
	}
}
