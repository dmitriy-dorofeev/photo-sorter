package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"photo-sorter/internal/sorter"
	"photo-sorter/internal/state"
)

func closeState(t *testing.T, res Result) {
	t.Helper()
	if res.State != nil {
		if err := res.State.Close(); err != nil {
			t.Logf("state close: %v", err)
		}
	}
}

func saveResultState(t *testing.T, res Result, targetDir string) {
	t.Helper()
	st, err := state.Open(targetDir)
	if err != nil {
		t.Fatalf("open state: %v", err)
	}
	records := make([]state.Record, 0, len(res.Entries))
	for _, e := range res.Entries {
		rec := state.Record{
			SourcePath: e.Source.Path,
			Size:       e.Source.Size,
			ModTime:    e.Source.ModTime,
			FastHash:   res.FastHashes[e.Source.Path],
			FullHash:   res.FullHashes[e.Source.Path],
			TargetPath: e.Target,
		}
		records = append(records, rec)
	}
	if err := st.Update(records); err != nil {
		t.Fatalf("state update: %v", err)
	}
	if err := st.Cleanup(res.AllPaths); err != nil {
		t.Fatalf("state cleanup: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("state close: %v", err)
	}
}

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
	defer closeState(t, res)

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
	defer closeState(t, res)

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
	defer closeState(t, res)
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
	defer closeState(t, res)

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

	res, err := Run(context.Background(), cfg, nil)
	closeState(t, res)
	if err == nil {
		t.Error("expected error for invalid file name template")
	}
}

func TestRun_Incremental_SecondRunSkipsAll(t *testing.T) {
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

	// Первый прогон.
	res1, err := Run(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	closeState(t, res1)
	saveResultState(t, res1, targetDir)

	// Второй прогон — все файлы должны быть отфильтрованы.
	res2, err := Run(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	defer closeState(t, res2)

	if len(res2.Files) != 12 {
		t.Errorf("expected 12 scanned files, got %d", len(res2.Files))
	}
	if len(res2.Entries) != 0 {
		t.Errorf("expected 0 entries on second run, got %d", len(res2.Entries))
	}
}

func TestRun_Incremental_NewFileOnly(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	// Создаём первый файл.
	f1 := filepath.Join(sourceDir, "a.jpg")
	if err := os.WriteFile(f1, []byte("photo-a"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cfg := Config{
		Sources:          []string{sourceDir},
		Target:           targetDir,
		Template:         "2006/01/02",
		FileNameTemplate: "{original}{ext}",
		LivePhotos:       true,
		IncludeVideo:     false,
		UseMTime:         true,
	}

	// Первый прогон.
	res1, err := Run(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	closeState(t, res1)
	saveResultState(t, res1, targetDir)

	// Добавляем второй файл.
	f2 := filepath.Join(sourceDir, "b.jpg")
	if err := os.WriteFile(f2, []byte("photo-b"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Второй прогон — должен обработать только новый файл.
	res2, err := Run(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	defer closeState(t, res2)

	if len(res2.Files) != 2 {
		t.Errorf("expected 2 scanned files, got %d", len(res2.Files))
	}
	if len(res2.Entries) != 1 {
		t.Errorf("expected 1 entry on second run, got %d", len(res2.Entries))
	}
}

func TestRun_FullCheck(t *testing.T) {
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

	// Первый прогон.
	res1, err := Run(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	closeState(t, res1)
	saveResultState(t, res1, targetDir)

	// Второй прогон с FullCheck — должен обработать все файлы снова.
	cfg.FullCheck = true
	res2, err := Run(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("second run (full-check) failed: %v", err)
	}
	defer closeState(t, res2)

	if len(res2.Entries) != 12 {
		t.Errorf("expected 12 entries on full-check run, got %d", len(res2.Entries))
	}
}
