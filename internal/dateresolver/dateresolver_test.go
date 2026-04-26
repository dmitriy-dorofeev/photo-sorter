package dateresolver

import (
	"os"
	"path/filepath"
	"photo-sorter/internal/scanner"
	"testing"
	"time"
)

// TestParseFromFilename_Patterns проверяет все известные паттерны имён файлов.
func TestParseFromFilename_Patterns(t *testing.T) {
	loc := time.Local

	tests := []struct {
		name     string
		filename string
		want     time.Time
	}{
		{
			name:     "Screenshot",
			filename: "Screenshot 2024-03-15 at 14.30.22.png",
			want:     time.Date(2024, 3, 15, 14, 30, 22, 0, loc),
		},
		{
			name:     "DateTimeWithDots",
			filename: "2024-03-15 14.30.22.jpg",
			want:     time.Date(2024, 3, 15, 14, 30, 22, 0, loc),
		},
		{
			name:     "IMG",
			filename: "IMG_20240315_143022.jpg",
			want:     time.Date(2024, 3, 15, 14, 30, 22, 0, loc),
		},
		{
			name:     "VID",
			filename: "VID_20240315_143022.mp4",
			want:     time.Date(2024, 3, 15, 14, 30, 22, 0, loc),
		},
		{
			name:     "PXL",
			filename: "PXL_20240315_143022123.jpg",
			want:     time.Date(2024, 3, 15, 14, 30, 22, 0, loc),
		},
		{
			name:     "Signal",
			filename: "signal-2024-03-15-14-30-22.jpg",
			want:     time.Date(2024, 3, 15, 14, 30, 22, 0, loc),
		},
		{
			name:     "IMG-WA",
			filename: "IMG-20240315-WA0001.jpg",
			want:     time.Date(2024, 3, 15, 0, 0, 0, 0, loc),
		},
		{
			name:     "PlainDateTime",
			filename: "20240315_143022.jpg",
			want:     time.Date(2024, 3, 15, 14, 30, 22, 0, loc),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseFromFilename(tt.filename)
			if !ok {
				t.Fatalf("parseFromFilename(%q) returned ok=false, want true", tt.filename)
			}
			if !got.Equal(tt.want) {
				t.Fatalf("parseFromFilename(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

// TestParseFromFilename_EdgeCases проверяет ситуации, когда имя файла
// не соответствует ни одному паттерну.
func TestParseFromFilename_EdgeCases(t *testing.T) {
	cases := []string{
		"",
		"photo.jpg",
		"DSC_1234.jpg",
		"IMG_1234.jpg",
		"random-text.png",
		"2024.jpg",
	}

	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			_, ok := parseFromFilename(name)
			if ok {
				t.Fatalf("parseFromFilename(%q) unexpectedly returned ok=true", name)
			}
		})
	}
}

// TestExtractExifDate проверяет чтение EXIF из готового тестового JPEG.
func TestExtractExifDate(t *testing.T) {
	path := "../../testdata/dateresolver/minimal.jpg"

	got, ok := extractExifDate(path)
	if !ok {
		t.Fatal("expected EXIF date to be extracted")
	}

	want := time.Date(2024, 3, 15, 14, 30, 22, 0, time.Local)
	if !got.Equal(want) {
		t.Fatalf("extractExifDate() = %v, want %v", got, want)
	}
}

// TestExtractExifDate_NoExif проверяет, что для файла без EXIF возвращается false.
func TestExtractExifDate_NoExif(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "no_exif.jpg")

	// Псевдо-JPEG без APP1
	data := []byte{0xFF, 0xD8, 0xFF, 0xD9}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, ok := extractExifDate(path)
	if ok {
		t.Fatal("expected no EXIF date for invalid file")
	}
}

// TestExtractExifDate_MissingFile проверяет обработку несуществующего файла.
func TestExtractExifDate_MissingFile(t *testing.T) {
	_, ok := extractExifDate("/nonexistent/path/file.jpg")
	if ok {
		t.Fatal("expected false for missing file")
	}
}

// TestResolve_Priority проверяет порядок приоритетов:
// EXIF > имя файла > ModTime.
func TestResolve_Priority(t *testing.T) {
	loc := time.Local
	fallback := time.Date(2023, 1, 1, 0, 0, 0, 0, loc)

	t.Run("filename beats mtime", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "IMG_20240315_143022.jpg")
		// Файл без валидного EXIF
		if err := os.WriteFile(path, []byte{0xFF, 0xD8, 0xFF, 0xD9}, 0644); err != nil {
			t.Fatal(err)
		}

		r := New()
		f := scanner.FileInfo{
			Path:    path,
			Name:    "IMG_20240315_143022.jpg",
			ModTime: fallback,
			Ext:     ".jpg",
		}

		got, ok := r.Resolve(f)
		if !ok {
			t.Fatal("expected date")
		}
		want := time.Date(2024, 3, 15, 14, 30, 22, 0, loc)
		if !got.Equal(want) {
			t.Fatalf("Resolve() = %v, want %v", got, want)
		}
	})

	t.Run("mtime fallback when enabled", func(t *testing.T) {
		r := New()
		r.UseModTime = true
		f := scanner.FileInfo{
			Path:    "/tmp/unknown.jpg",
			Name:    "unknown.jpg",
			ModTime: fallback,
			Ext:     ".jpg",
		}

		got, ok := r.Resolve(f)
		if !ok {
			t.Fatal("expected mtime fallback")
		}
		if !got.Equal(fallback) {
			t.Fatalf("Resolve() = %v, want %v (mtime)", got, fallback)
		}
	})

	t.Run("mtime fallback can be disabled", func(t *testing.T) {
		r := New()
		r.UseModTime = false
		f := scanner.FileInfo{
			Path:    "/tmp/unknown.jpg",
			Name:    "unknown.jpg",
			ModTime: fallback,
			Ext:     ".jpg",
		}

		_, ok := r.Resolve(f)
		if ok {
			t.Fatal("expected false when UseModTime is false and no EXIF/filename date")
		}
	})

	t.Run("unsorted when nothing works", func(t *testing.T) {
		r := New()
		f := scanner.FileInfo{
			Path:    "/tmp/unknown.jpg",
			Name:    "unknown.jpg",
			ModTime: time.Time{}, // zero
			Ext:     ".jpg",
		}

		_, ok := r.Resolve(f)
		if ok {
			t.Fatal("expected false when no date source available")
		}
	})
}
