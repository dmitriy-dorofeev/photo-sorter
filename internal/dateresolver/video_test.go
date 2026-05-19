package dateresolver

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"photo-sorter/internal/scanner"
)

// writeFakeExifTool создаёт временный фейковый exiftool-скрипт
// и возвращает путь к нему.
func writeFakeExifTool(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	var name string
	if runtime.GOOS == "windows" {
		name = "fake_exiftool.bat"
	} else {
		name = "fake_exiftool.sh"
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractVideoDate_Success(t *testing.T) {
	script := `#!/bin/sh
echo '[{"SourceFile":"'$3'","CreateDate":"2024:06:15 10:20:30"}]'
`
	fake := writeFakeExifTool(t, script)

	got, ok := extractVideoDate(context.Background(), "/fake/video.mp4", fake)
	if !ok {
		t.Fatal("expected ok=true")
	}
	want := time.Date(2024, 6, 15, 10, 20, 30, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExtractVideoDate_Priority(t *testing.T) {
	// DateTimeOriginal должна побеждать CreateDate.
	script := `#!/bin/sh
echo '[{"SourceFile":"'$3'","DateTimeOriginal":"2023:01:01 00:00:01","CreateDate":"2024:12:31 23:59:59"}]'
`
	fake := writeFakeExifTool(t, script)

	got, ok := extractVideoDate(context.Background(), "/fake/video.mp4", fake)
	if !ok {
		t.Fatal("expected ok=true")
	}
	want := time.Date(2023, 1, 1, 0, 0, 1, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExtractVideoDate_MediaCreateDateFallback(t *testing.T) {
	script := `#!/bin/sh
echo '[{"SourceFile":"'$3'","MediaCreateDate":"2022:05:05 12:00:00"}]'
`
	fake := writeFakeExifTool(t, script)

	got, ok := extractVideoDate(context.Background(), "/fake/video.mp4", fake)
	if !ok {
		t.Fatal("expected ok=true")
	}
	want := time.Date(2022, 5, 5, 12, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExtractVideoDate_NoDate(t *testing.T) {
	script := `#!/bin/sh
echo '[{"SourceFile":"'$3'"}]'
`
	fake := writeFakeExifTool(t, script)

	_, ok := extractVideoDate(context.Background(), "/fake/video.mp4", fake)
	if ok {
		t.Fatal("expected ok=false when no date fields present")
	}
}

func TestExtractVideoDate_NotFound(t *testing.T) {
	_, ok := extractVideoDate(context.Background(), "/fake/video.mp4", "/nonexistent/exiftool")
	if ok {
		t.Fatal("expected ok=false when exiftool not found")
	}
}

func TestExtractVideoDate_InvalidJSON(t *testing.T) {
	script := `#!/bin/sh
echo 'not json'
`
	fake := writeFakeExifTool(t, script)

	_, ok := extractVideoDate(context.Background(), "/fake/video.mp4", fake)
	if ok {
		t.Fatal("expected ok=false when exiftool returns invalid JSON")
	}
}

func TestExtractVideoDate_FlagInjection(t *testing.T) {
	// Имя файла начинается с "-" — без "--" exiftool интерпретировал бы его как флаг.
	script := `#!/bin/sh
echo '[{"SourceFile":"'$5'","CreateDate":"2024:06:15 10:20:30"}]'
`
	fake := writeFakeExifTool(t, script)

	got, ok := extractVideoDate(context.Background(), "-overwrite_original.mp4", fake)
	if !ok {
		t.Fatal("expected ok=true even for path starting with dash")
	}
	want := time.Date(2024, 6, 15, 10, 20, 30, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestResolve_VideoPriority(t *testing.T) {
	// Видео с метаданными exiftool должно получить дату оттуда,
	// а не из имени файла (даже если имя парсится).
	script := `#!/bin/sh
echo '[{"SourceFile":"'$3'","CreateDate":"2021:07:07 07:07:07"}]'
`
	fake := writeFakeExifTool(t, script)

	r := &Resolver{ExifToolPath: fake}
	f := scanner.FileInfo{
		Path:    "/tmp/VID_20240520_120000.mp4",
		Name:    "VID_20240520_120000.mp4",
		Ext:     ".mp4",
		ModTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	got, ok := r.Resolve(context.Background(), f)
	if !ok {
		t.Fatal("expected ok=true")
	}
	want := time.Date(2021, 7, 7, 7, 7, 7, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v (exiftool should beat filename)", got, want)
	}
}

func TestResolve_VideoFallbackToFilename(t *testing.T) {
	// Видео без метаданных exiftool, но с парсимым именем.
	script := `#!/bin/sh
echo '[{"SourceFile":"'$3'"}]'
`
	fake := writeFakeExifTool(t, script)

	r := &Resolver{ExifToolPath: fake}
	f := scanner.FileInfo{
		Path:    "/tmp/VID_20240520_120000.mp4",
		Name:    "VID_20240520_120000.mp4",
		Ext:     ".mp4",
		ModTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	got, ok := r.Resolve(context.Background(), f)
	if !ok {
		t.Fatal("expected ok=true")
	}
	want := time.Date(2024, 5, 20, 12, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v (filename fallback)", got, want)
	}
}

func TestResolve_VideoFallbackToModTime(t *testing.T) {
	// Видео без метаданных и без парсимого имени, но с UseModTime.
	script := `#!/bin/sh
echo '[{"SourceFile":"'$3'"}]'
`
	fake := writeFakeExifTool(t, script)

	r := &Resolver{ExifToolPath: fake, UseModTime: true}
	f := scanner.FileInfo{
		Path:    "/tmp/video.mp4",
		Name:    "video.mp4",
		Ext:     ".mp4",
		ModTime: time.Date(2024, 3, 15, 14, 30, 0, 0, time.UTC),
	}

	got, ok := r.Resolve(context.Background(), f)
	if !ok {
		t.Fatal("expected ok=true")
	}
	want := time.Date(2024, 3, 15, 14, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v (mtime fallback)", got, want)
	}
}

func TestParseExifDate_VariousLayouts(t *testing.T) {
	tests := []struct {
		input string
		want  time.Time
		ok    bool
	}{
		{"2024:06:15 10:20:30", time.Date(2024, 6, 15, 10, 20, 30, 0, time.UTC), true},
		{"2024:06:15 10:20:30+05:00", time.Date(2024, 6, 15, 10, 20, 30, 0, time.FixedZone("", 5*60*60)), true},
		{"2024:06:15 10:20:30.123", time.Date(2024, 6, 15, 10, 20, 30, 123000000, time.UTC), true},
		{"2024:06:15 10:20:30.123-07:00", time.Date(2024, 6, 15, 10, 20, 30, 123000000, time.FixedZone("", -7*60*60)), true},
		{"2024:06:15 10:20:30Z", time.Date(2024, 6, 15, 10, 20, 30, 0, time.UTC), true},
		{"invalid", time.Time{}, false},
		{"", time.Time{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := parseExifDate(tt.input)
			if ok != tt.ok {
				t.Fatalf("parseExifDate(%q) ok=%v, want %v", tt.input, ok, tt.ok)
			}
			if tt.ok && !got.Equal(tt.want) {
				t.Errorf("parseExifDate(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
