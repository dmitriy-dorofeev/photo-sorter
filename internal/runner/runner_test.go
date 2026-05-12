package runner

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"photo-sorter/internal/sorter"
)

func TestRun_EndToEnd(t *testing.T) {
	sourceDir := filepath.Join("..", "..", "testdata", "e2e", "source", "2023")
	targetDir := t.TempDir()

	cfg := Config{
		Sources:          []string{sourceDir},
		Target:           targetDir,
		Template:         "2006/01/02",
		FileNameTemplate: "{original}{ext}",
		LivePhotos:       true,
		IncludeVideo:     true,
		UseMTime:         false,
	}

	res, err := Run(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(res.Files) != 12 {
		t.Errorf("expected 12 files, got %d", len(res.Files))
	}
	if len(res.Entries) != 12 {
		t.Errorf("expected 12 entries, got %d", len(res.Entries))
	}

	// Verify duplicates found (minimal.jpg copies)
	var dupCount int
	for _, g := range res.Duplicates {
		dupCount += len(g.Duplicates)
	}
	if dupCount < 5 {
		t.Errorf("expected at least 5 duplicates (minimal.jpg copies), got %d", dupCount)
	}

	// Verify live_photo files are NOT duplicates
	for _, g := range res.Duplicates {
		for _, d := range g.Duplicates {
			if strings.Contains(d.Name, "live_photo") {
				t.Errorf("live_photo file marked as duplicate: %s", d.Name)
			}
		}
	}

	// Verify unsorted count: live_photo.HEIC, live_photo.MOV, video.mp4, photo_no_date.jpg = 4
	var unsortedCount int
	for _, e := range res.Entries {
		if !e.Skip && sorter.IsUnsorted(e.Target) {
			unsortedCount++
		}
	}
	if unsortedCount != 4 {
		t.Errorf("expected 4 unsorted, got %d", unsortedCount)
	}
}

func TestRun_UseModTime(t *testing.T) {
	sourceDir := filepath.Join("..", "..", "testdata", "e2e", "source", "2023")
	targetDir := t.TempDir()

	cfg := Config{
		Sources:          []string{sourceDir},
		Target:           targetDir,
		Template:         "2006/01/02",
		FileNameTemplate: "{original}{ext}",
		LivePhotos:       true,
		IncludeVideo:     true,
		UseMTime:         true,
	}

	res, err := Run(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// With UseModTime=true, nothing should be unsorted
	var unsortedCount int
	for _, e := range res.Entries {
		if !e.Skip && sorter.IsUnsorted(e.Target) {
			unsortedCount++
		}
	}
	if unsortedCount != 0 {
		t.Errorf("expected 0 unsorted with UseModTime=true, got %d", unsortedCount)
	}
}

func TestRun_Progress(t *testing.T) {
	sourceDir := filepath.Join("..", "..", "testdata", "e2e", "source", "2023")
	targetDir := t.TempDir()

	cfg := Config{
		Sources:          []string{sourceDir},
		Target:           targetDir,
		Template:         "2006/01/02",
		FileNameTemplate: "{original}{ext}",
		LivePhotos:       true,
		IncludeVideo:     true,
	}

	var stages []string
	res, err := Run(context.Background(), cfg, func(stage string, current, total int) {
		stages = append(stages, stage)
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if len(res.Files) == 0 {
		t.Error("expected files")
	}
	if len(stages) != 3 {
		t.Errorf("expected 3 progress stages, got %v", stages)
	}
	if stages[0] != "scan" || stages[1] != "dedup" || stages[2] != "sort" {
		t.Errorf("unexpected stages: %v", stages)
	}
}

func TestRun_DupStrategies(t *testing.T) {
	sourceDir := filepath.Join("..", "..", "testdata", "e2e", "source", "2023")

	strategies := []string{"path", "largest", "newest", "best-meta"}
	for _, s := range strategies {
		t.Run(s, func(t *testing.T) {
			targetDir := t.TempDir()
			cfg := Config{
				Sources:          []string{sourceDir},
				Target:           targetDir,
				Template:         "2006/01/02",
				FileNameTemplate: "{original}{ext}",
				LivePhotos:       true,
				IncludeVideo:     true,
				UseMTime:         false,
				DupStrategy:      s,
			}

			res, err := Run(context.Background(), cfg, nil)
			if err != nil {
				t.Fatalf("Run failed with strategy=%s: %v", s, err)
			}
			if len(res.Files) != 12 {
				t.Errorf("expected 12 files, got %d", len(res.Files))
			}

			var dupCount int
			for _, g := range res.Duplicates {
				dupCount += len(g.Duplicates)
			}
			if dupCount < 5 {
				t.Errorf("expected at least 5 duplicates, got %d", dupCount)
			}

			// Убеждаемся, что live_photo файлы не помечены как дубликаты друг друга
			for _, g := range res.Duplicates {
				for _, d := range g.Duplicates {
					if strings.Contains(d.Name, "live_photo") {
						t.Errorf("live_photo file marked as duplicate: %s", d.Name)
					}
				}
			}
		})
	}
}

func TestRun_WithFileNameTemplate(t *testing.T) {
	sourceDir := filepath.Join("..", "..", "testdata", "e2e", "source", "2023")
	targetDir := t.TempDir()

	cfg := Config{
		Sources:          []string{sourceDir},
		Target:           targetDir,
		Template:         "2006/01/02",
		FileNameTemplate: "{YYYY}-{MM}-{DD}_{original}{ext}",
		LivePhotos:       true,
		IncludeVideo:     true,
		UseMTime:         false,
	}

	res, err := Run(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Проверяем, что имена файлов сгенерированы по шаблону
	var foundGenerated bool
	for _, e := range res.Entries {
		if !e.Skip && !sorter.IsUnsorted(e.Target) {
			base := filepath.Base(e.Target)
			if strings.HasPrefix(base, "2024-") || strings.HasPrefix(base, "2023-") {
				foundGenerated = true
			}
		}
	}
	if !foundGenerated {
		t.Error("expected at least one file with generated name")
	}
}

func TestRun_InvalidFileNameTemplate(t *testing.T) {
	sourceDir := filepath.Join("..", "..", "testdata", "e2e", "source", "2023")
	targetDir := t.TempDir()

	cfg := Config{
		Sources:          []string{sourceDir},
		Target:           targetDir,
		Template:         "2006/01/02",
		FileNameTemplate: "{BAD}",
		LivePhotos:       true,
		IncludeVideo:     true,
	}

	_, err := Run(context.Background(), cfg, nil)
	if err == nil {
		t.Error("expected error for invalid file name template")
	}
}
