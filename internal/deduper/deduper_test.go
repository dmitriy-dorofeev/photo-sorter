package deduper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"photo-sorter/internal/dateresolver"
	"photo-sorter/internal/hasher"
	"photo-sorter/internal/scanner"

	"golang.org/x/sys/unix"
)

func testdata(name string) string {
	return filepath.Join("..", "..", "testdata", "deduper", name)
}

func TestHashFile_Stability(t *testing.T) {
	path := testdata("dup_a.bin")
	h1, err := hasher.HashFile(context.Background(), path)
	if err != nil {
		t.Fatalf("hashFile first run: %v", err)
	}
	h2, err := hasher.HashFile(context.Background(), path)
	if err != nil {
		t.Fatalf("hashFile second run: %v", err)
	}
	if h1 != h2 {
		t.Errorf("hash not stable: %d != %d", h1, h2)
	}
}

func TestHashFile_DifferentContent(t *testing.T) {
	h1, err := hasher.HashFile(context.Background(), testdata("same_size_a.bin"))
	if err != nil {
		t.Fatalf("hashFile: %v", err)
	}
	h2, err := hasher.HashFile(context.Background(), testdata("same_size_b.bin"))
	if err != nil {
		t.Fatalf("hashFile: %v", err)
	}
	if h1 == h2 {
		t.Error("expected different hashes for different content")
	}
}

func TestFindDuplicates(t *testing.T) {
	tests := []struct {
		name    string
		files   []scanner.FileInfo
		wantLen int
		check   func(t *testing.T, results []Result)
	}{
		{
			name:    "empty input",
			files:   nil,
			wantLen: 0,
		},
		{
			name: "single file",
			files: []scanner.FileInfo{
				{Path: testdata("single.bin"), Name: "single.bin", Size: 200},
			},
			wantLen: 0,
		},
		{
			name: "no duplicates different sizes",
			files: []scanner.FileInfo{
				{Path: testdata("single.bin"), Name: "single.bin", Size: 200},
				{Path: testdata("dup_a.bin"), Name: "dup_a.bin", Size: 100},
			},
			wantLen: 0,
		},
		{
			name: "exact duplicates",
			files: []scanner.FileInfo{
				{Path: testdata("dup_a.bin"), Name: "dup_a.bin", Size: 100},
				{Path: testdata("dup_b.bin"), Name: "dup_b.bin", Size: 100},
			},
			wantLen: 1,
			check: func(t *testing.T, results []Result) {
				if len(results[0].Duplicates) != 1 {
					t.Errorf("expected 1 duplicate, got %d", len(results[0].Duplicates))
				}
				if results[0].Original.Name != "dup_a.bin" {
					t.Errorf("expected original dup_a.bin, got %s", results[0].Original.Name)
				}
				if results[0].Duplicates[0].Name != "dup_b.bin" {
					t.Errorf("expected duplicate dup_b.bin, got %s", results[0].Duplicates[0].Name)
				}
			},
		},
		{
			name: "same size different content",
			files: []scanner.FileInfo{
				{Path: testdata("same_size_a.bin"), Name: "same_size_a.bin", Size: 100},
				{Path: testdata("same_size_b.bin"), Name: "same_size_b.bin", Size: 100},
			},
			wantLen: 0,
		},
		{
			name: "live photos not duplicates",
			files: []scanner.FileInfo{
				{Path: testdata("live_photo.heic"), Name: "live_photo.heic", Ext: ".heic", Size: 50},
				{Path: testdata("live_photo.mov"), Name: "live_photo.mov", Ext: ".mov", Size: 50},
			},
			wantLen: 0,
		},
		{
			name: "live photos disabled treats as duplicates",
			files: []scanner.FileInfo{
				// Используем dup_a.bin и dup_b.bin (одинаковое содержимое), но с именами Live Photos
				{Path: testdata("dup_a.bin"), Name: "live_photo.heic", Ext: ".heic", Size: 100},
				{Path: testdata("dup_b.bin"), Name: "live_photo.mov", Ext: ".mov", Size: 100},
			},
			wantLen: 1,
			check: func(t *testing.T, results []Result) {
				if results[0].Original.Name != "live_photo.heic" {
					t.Errorf("expected original live_photo.heic, got %s", results[0].Original.Name)
				}
				if len(results[0].Duplicates) != 1 || results[0].Duplicates[0].Name != "live_photo.mov" {
					t.Errorf("expected duplicate live_photo.mov, got %v", results[0].Duplicates)
				}
			},
		},
		{
			name: "live photos with duplicate heic",
			files: []scanner.FileInfo{
				{Path: testdata("live_photo.heic"), Name: "live_photo.heic", Ext: ".heic", Size: 50},
				{Path: testdata("live_photo.mov"), Name: "live_photo.mov", Ext: ".mov", Size: 50},
				{Path: testdata("live_photo_copy.heic"), Name: "live_photo_copy.heic", Ext: ".heic", Size: 50},
			},
			wantLen: 1,
			check: func(t *testing.T, results []Result) {
				// Original should be one of the .heic files
				orig := results[0].Original.Name
				if orig != "live_photo.heic" && orig != "live_photo_copy.heic" {
					t.Errorf("unexpected original: %s", orig)
				}
				// Duplicate should be the other .heic file
				for _, d := range results[0].Duplicates {
					if d.Name == "live_photo.mov" {
						t.Error("live_photo.mov should not be a duplicate")
					}
				}
				if len(results[0].Duplicates) != 1 {
					t.Errorf("expected 1 duplicate, got %d", len(results[0].Duplicates))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			livePhotos := true
			if tt.name == "live photos disabled treats as duplicates" {
				livePhotos = false
			}
			d := New(tt.files, livePhotos, StrategyPath, nil)
			got, err := d.FindDuplicates(context.Background())
			if err != nil {
				t.Fatalf("FindDuplicates() error = %v", err)
			}
			if len(got) != tt.wantLen {
				t.Errorf("FindDuplicates() returned %d results, want %d", len(got), tt.wantLen)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestFindDuplicates_HashError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}

	dir := t.TempDir()
	noRead := filepath.Join(dir, "secret.bin")
	os.WriteFile(noRead, []byte("secret"), 0644)
	os.Chmod(noRead, 0000)
	defer os.Chmod(noRead, 0644)

	// Одинаковый размер, чтобы оба файла попали в группу хеширования.
	files := []scanner.FileInfo{
		{Path: noRead, Name: "secret.bin", Size: 100},
		{Path: testdata("dup_a.bin"), Name: "dup_a.bin", Size: 100},
	}
	d := New(files, true, StrategyPath, nil)
	results, err := d.FindDuplicates(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// Нехешируемый файл исключается из дедупликации — остаётся один файл в группе.
	if len(results) != 0 {
		t.Errorf("expected 0 duplicates, got %d", len(results))
	}
}

func TestFindDuplicates_NamedPipe(t *testing.T) {
	dir := t.TempDir()
	pipePath := filepath.Join(dir, "pipe.bin")
	if err := unix.Mkfifo(pipePath, 0644); err != nil {
		t.Skipf("cannot create fifo: %v", err)
	}

	// Одинаковый размер, чтобы оба файла попали в группу хеширования.
	files := []scanner.FileInfo{
		{Path: pipePath, Name: "pipe.bin", Size: 100},
		{Path: testdata("dup_a.bin"), Name: "dup_a.bin", Size: 100},
	}
	d := New(files, true, StrategyPath, nil)
	results, err := d.FindDuplicates(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// Non-regular файл исключается из дедупликации — остаётся один файл в группе.
	if len(results) != 0 {
		t.Errorf("expected 0 duplicates, got %d", len(results))
	}
}

func TestFindDuplicates_Strategies(t *testing.T) {
	files := []scanner.FileInfo{
		{Path: testdata("dup_a.bin"), Name: "dup_a.bin", Size: 100},
		{Path: testdata("dup_b.bin"), Name: "dup_b.bin", Size: 100},
	}

	tests := []struct {
		name        string
		strategy    Strategy
		dateSources map[string]dateresolver.Source
		wantOrig    string
	}{
		{
			name:     "path strategy picks alphabetical",
			strategy: StrategyPath,
			wantOrig: "dup_a.bin",
		},
		{
			name:     "largest strategy falls back to path when sizes equal",
			strategy: StrategyLargest,
			wantOrig: "dup_a.bin",
		},
		{
			name:     "newest strategy falls back to path when times equal",
			strategy: StrategyNewest,
			wantOrig: "dup_a.bin",
		},
		{
			name:     "best-meta picks higher source",
			strategy: StrategyBestMeta,
			dateSources: map[string]dateresolver.Source{
				testdata("dup_a.bin"): dateresolver.SourceExif,
				testdata("dup_b.bin"): dateresolver.SourceFilename,
			},
			wantOrig: "dup_a.bin",
		},
		{
			name:        "best-meta with nil map falls back to largest then path",
			strategy:    StrategyBestMeta,
			dateSources: nil,
			wantOrig:    "dup_a.bin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := New(files, true, tt.strategy, tt.dateSources)
			results, err := d.FindDuplicates(context.Background())
			if err != nil {
				t.Fatalf("FindDuplicates() error = %v", err)
			}
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}
			if results[0].Original.Name != tt.wantOrig {
				t.Errorf("expected original %s, got %s", tt.wantOrig, results[0].Original.Name)
			}
			if len(results[0].Duplicates) != 1 {
				t.Errorf("expected 1 duplicate, got %d", len(results[0].Duplicates))
			}
		})
	}
}

func BenchmarkFindDuplicates(b *testing.B) {
	dir := b.TempDir()
	var files []scanner.FileInfo
	// 100 файлов: каждые два — дубликаты.
	for i := 0; i < 100; i++ {
		path := filepath.Join(dir, fmt.Sprintf("file%03d.bin", i))
		content := []byte(fmt.Sprintf("content-%d", i/2))
		os.WriteFile(path, content, 0644)
		files = append(files, scanner.FileInfo{
			Path: path,
			Name: fmt.Sprintf("file%03d.bin", i),
			Size: int64(len(content)),
		})
	}
	d := New(files, true, StrategyPath, nil)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := d.FindDuplicates(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}
