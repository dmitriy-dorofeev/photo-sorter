package deduper

import (
	"path/filepath"
	"testing"

	"photo-sorter/internal/scanner"
)

func testdata(name string) string {
	return filepath.Join("..", "..", "testdata", "deduper", name)
}

func TestHashFile_Stability(t *testing.T) {
	path := testdata("dup_a.bin")
	h1, err := hashFile(path)
	if err != nil {
		t.Fatalf("hashFile first run: %v", err)
	}
	h2, err := hashFile(path)
	if err != nil {
		t.Fatalf("hashFile second run: %v", err)
	}
	if h1 != h2 {
		t.Errorf("hash not stable: %d != %d", h1, h2)
	}
}

func TestHashFile_DifferentContent(t *testing.T) {
	h1, err := hashFile(testdata("same_size_a.bin"))
	if err != nil {
		t.Fatalf("hashFile: %v", err)
	}
	h2, err := hashFile(testdata("same_size_b.bin"))
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
			d := New(tt.files)
			got := d.FindDuplicates()
			if len(got) != tt.wantLen {
				t.Errorf("FindDuplicates() returned %d results, want %d", len(got), tt.wantLen)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}
