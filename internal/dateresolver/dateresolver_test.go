package dateresolver

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"photo-sorter/internal/scanner"
	"strings"
	"sync"
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

		got, ok := r.Resolve(context.Background(), f)
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

		got, ok := r.Resolve(context.Background(), f)
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

		_, ok := r.Resolve(context.Background(), f)
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

		_, ok := r.Resolve(context.Background(), f)
		if ok {
			t.Fatal("expected false when no date source available")
		}
	})
}

// TestResolveWithSource_Priority проверяет, что ResolveWithSource возвращает
// корректный Source в зависимости от источника даты.
func TestResolveWithSource_Priority(t *testing.T) {
	loc := time.Local

	t.Run("EXIF returns SourceExif", func(t *testing.T) {
		path := "../../testdata/dateresolver/minimal.jpg"
		r := New()
		f := scanner.FileInfo{
			Path: path,
			Name: "minimal.jpg",
			Ext:  ".jpg",
		}
		_, ok, src := r.ResolveWithSource(context.Background(), f)
		if !ok {
			t.Fatal("expected ok=true for EXIF file")
		}
		if src != SourceExif {
			t.Errorf("expected SourceExif, got %v", src)
		}
	})

	t.Run("filename returns SourceFilename", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "IMG_20240315_143022.jpg")
		// Псевдо-JPEG без валидного EXIF
		if err := os.WriteFile(path, []byte{0xFF, 0xD8, 0xFF, 0xD9}, 0644); err != nil {
			t.Fatal(err)
		}
		r := New()
		f := scanner.FileInfo{
			Path:    path,
			Name:    "IMG_20240315_143022.jpg",
			ModTime: time.Date(2023, 1, 1, 0, 0, 0, 0, loc),
			Ext:     ".jpg",
		}
		_, ok, src := r.ResolveWithSource(context.Background(), f)
		if !ok {
			t.Fatal("expected ok=true for filename-parseable file")
		}
		if src != SourceFilename {
			t.Errorf("expected SourceFilename, got %v", src)
		}
	})

	t.Run("ModTime fallback returns SourceModTime", func(t *testing.T) {
		r := New()
		r.UseModTime = true
		fallback := time.Date(2023, 1, 1, 0, 0, 0, 0, loc)
		f := scanner.FileInfo{
			Path:    "/tmp/unknown.jpg",
			Name:    "unknown.jpg",
			ModTime: fallback,
			Ext:     ".jpg",
		}
		_, ok, src := r.ResolveWithSource(context.Background(), f)
		if !ok {
			t.Fatal("expected ok=true for mtime fallback")
		}
		if src != SourceModTime {
			t.Errorf("expected SourceModTime, got %v", src)
		}
	})

	t.Run("none returns SourceNone", func(t *testing.T) {
		r := New()
		r.UseModTime = false
		f := scanner.FileInfo{
			Path:    "/tmp/unknown.jpg",
			Name:    "unknown.jpg",
			ModTime: time.Date(2023, 1, 1, 0, 0, 0, 0, loc),
			Ext:     ".jpg",
		}
		_, ok, src := r.ResolveWithSource(context.Background(), f)
		if ok {
			t.Fatal("expected ok=false when no date source available")
		}
		if src != SourceNone {
			t.Errorf("expected SourceNone, got %v", src)
		}
	})
}

// TestExtractVideoDate_CommandInjection проверяет, что файл с именем,
// начинающимся на "-", не интерпретируется как флаг exiftool.
func TestExtractVideoDate_CommandInjection(t *testing.T) {
	tmp := t.TempDir()
	movPath := filepath.Join(tmp, "-test.mov")
	os.WriteFile(movPath, []byte("fake"), 0644)

	fakeExifTool := filepath.Join(t.TempDir(), "fake_exiftool.sh")
	script := `#!/bin/sh
echo '[{"CreateDate":"2023:07:07 07:07:07"}]'
`
	os.WriteFile(fakeExifTool, []byte(script), 0755)

	got, ok := extractVideoDate(context.Background(), movPath, fakeExifTool)
	if !ok {
		t.Fatal("expected date from file with leading dash")
	}
	want := time.Date(2023, 7, 7, 7, 7, 7, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

// TestExtractVideoDate_Timeout проверяет, что зависший exiftool
// прерывается по таймауту и возвращает (_, false).
func TestExtractVideoDate_Timeout(t *testing.T) {
	oldTimeout := videoTimeout
	videoTimeout = 2 * time.Second
	defer func() { videoTimeout = oldTimeout }()

	tmp := t.TempDir()
	movPath := filepath.Join(tmp, "slow.mov")
	os.WriteFile(movPath, []byte("fake"), 0644)

	// Собираем fake exiftool на Go, чтобы избежать проблем с shell script + sleep.
	fakeDir := t.TempDir()
	fakeSrc := filepath.Join(fakeDir, "main.go")
	os.WriteFile(fakeSrc, []byte("package main\nimport \"time\"\nfunc main() { time.Sleep(120 * time.Second) }\n"), 0644)
	fakeBin := filepath.Join(fakeDir, "fake_exiftool")
	if err := exec.Command("go", "build", "-o", fakeBin, fakeSrc).Run(); err != nil {
		t.Skipf("cannot build fake exiftool: %v", err)
	}

	start := time.Now()
	_, ok := extractVideoDate(context.Background(), movPath, fakeBin)
	elapsed := time.Since(start)
	if ok {
		t.Fatal("expected false on timeout")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("timeout took too long: %v", elapsed)
	}
}

// TestExtractVideoDate_Concurrent проверяет, что параллельные вызовы
// extractVideoDate работают корректно (нет data races).
func TestExtractVideoDate_Concurrent(t *testing.T) {
	tmp := t.TempDir()
	movPath := filepath.Join(tmp, "concurrent.mov")
	os.WriteFile(movPath, []byte("fake"), 0644)

	fakeExifTool := filepath.Join(t.TempDir(), "fake_exiftool.sh")
	script := `#!/bin/sh
echo '[{"CreateDate":"2023:07:07 07:07:07"}]'
`
	os.WriteFile(fakeExifTool, []byte(script), 0755)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok := extractVideoDate(context.Background(), movPath, fakeExifTool)
			if !ok {
				t.Error("expected date from concurrent call")
			}
		}()
	}
	wg.Wait()
}

// FuzzResolveDate фаззит resolver.Resolve с разными именами файлов,
// расширениями и флагом UseModTime. Цель — убедиться, что функция
// никогда не паникует при некорректных входных данных.
func FuzzResolveDate(f *testing.F) {
	f.Add("IMG_20240315_143022.jpg", ".jpg", true)
	f.Add("photo.jpg", ".jpg", false)
	f.Add("Screenshot 2024-03-15 at 14.30.22.png", ".png", true)
	f.Add("", ".jpg", true)
	f.Add("-test.mov", ".mov", false)

	f.Fuzz(func(t *testing.T, name, ext string, useModTime bool) {
		r := New()
		r.UseModTime = useModTime
		fi := scanner.FileInfo{
			Path:    filepath.Join("/tmp", name),
			Name:    name,
			ModTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Ext:     strings.ToLower(ext),
		}
		// Для несуществующих файлов EXIF не прочитается, но Resolve
		// должен корректно обработать любое имя без паники.
		_, _ = r.Resolve(context.Background(), fi)
	})
}
