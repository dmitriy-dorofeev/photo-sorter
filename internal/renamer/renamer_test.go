package renamer

import (
	"path/filepath"
	"testing"
	"time"

	"photo-sorter/internal/scanner"
)

func mustParse(t *testing.T, tmpl string) *Template {
	t.Helper()
	pt, err := Parse(tmpl)
	if err != nil {
		t.Fatalf("Parse(%q) failed: %v", tmpl, err)
	}
	return pt
}

func TestParse_Valid(t *testing.T) {
	cases := []string{
		"{YYYY}-{MM}-{DD}_{original}{ext}",
		"{original}{ext}",
		"{seq:03}{ext}",
		"photo_{YYYY}_{original}{ext}",
		"{device}_{original}{ext}",
	}
	for _, c := range cases {
		_, err := Parse(c)
		if err != nil {
			t.Errorf("Parse(%q) failed: %v", c, err)
		}
	}
}

func TestParse_InvalidPlaceholder(t *testing.T) {
	_, err := Parse("{INVALID}")
	if err == nil {
		t.Error("expected error for invalid placeholder")
	}
}

func TestParse_Empty(t *testing.T) {
	_, err := Parse("")
	if err == nil {
		t.Error("expected error for empty template")
	}
}

func TestParse_Unclosed(t *testing.T) {
	_, err := Parse("{YYYY")
	if err == nil {
		t.Error("expected error for unclosed placeholder")
	}
}

func TestParse_InvalidSeqWidth(t *testing.T) {
	cases := []string{"{seq:0}", "{seq:abc}", "{seq:-1}"}
	for _, c := range cases {
		_, err := Parse(c)
		if err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}

func TestRender_FullDateTime(t *testing.T) {
	tmpl := mustParse(t, "{YYYY}{MM}{DD}_{HH}{mm}{SS}{ext}")
	date := time.Date(2024, 3, 15, 14, 30, 22, 0, time.UTC)
	info := scanner.FileInfo{Name: "a.jpg"}
	got := tmpl.Render(date, info, 0)
	want := "20240315_143022.jpg"
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestRender_DateWithOriginal(t *testing.T) {
	tmpl := mustParse(t, "{YYYY}-{MM}-{DD}_{original}{ext}")
	date := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	info := scanner.FileInfo{Name: "IMG_1234.HEIC"}
	got := tmpl.Render(date, info, 0)
	want := "2024-03-15_IMG_1234.HEIC"
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestRender_OnlyOriginal(t *testing.T) {
	tmpl := mustParse(t, "{original}{ext}")
	info := scanner.FileInfo{Name: "photo.jpg"}
	got := tmpl.Render(time.Time{}, info, 0)
	want := "photo.jpg"
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestRender_PreservesExtCase(t *testing.T) {
	tmpl := mustParse(t, "{original}{ext}")
	info := scanner.FileInfo{Name: "photo.JPG"}
	got := tmpl.Render(time.Time{}, info, 0)
	want := "photo.JPG"
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestRender_SeqNoPadding(t *testing.T) {
	tmpl := mustParse(t, "{seq}{ext}")
	info := scanner.FileInfo{Name: "a.jpg"}
	got := tmpl.Render(time.Time{}, info, 5)
	want := "5.jpg"
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestRender_SeqWithPadding(t *testing.T) {
	tmpl := mustParse(t, "{seq:03}{ext}")
	info := scanner.FileInfo{Name: "a.jpg"}
	got := tmpl.Render(time.Time{}, info, 5)
	want := "005.jpg"
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestRender_SeqWithPadding_Boundary(t *testing.T) {
	tmpl := mustParse(t, "{seq:03}{ext}")
	info := scanner.FileInfo{Name: "a.jpg"}
	got := tmpl.Render(time.Time{}, info, 999)
	want := "999.jpg"
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestRender_SeqWithPadding_Overflow(t *testing.T) {
	tmpl := mustParse(t, "{seq:03}{ext}")
	info := scanner.FileInfo{Name: "a.jpg"}
	got := tmpl.Render(time.Time{}, info, 1000)
	want := "1000.jpg"
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestRender_ZeroDate_Unsorted(t *testing.T) {
	tmpl := mustParse(t, "{YYYY}-{MM}-{DD}_{original}{ext}")
	info := scanner.FileInfo{Name: "IMG_1234.jpg"}
	got := tmpl.Render(time.Time{}, info, 0)
	want := "0000-00-00_IMG_1234.jpg"
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestRender_DevicePlaceholder(t *testing.T) {
	tmpl := mustParse(t, "{device}_{original}{ext}")
	info := scanner.FileInfo{Name: "IMG_1234.jpg", Device: "iPhone"}
	got := tmpl.Render(time.Time{}, info, 0)
	want := "iPhone_IMG_1234.jpg"
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestRender_MixedStaticAndDynamic(t *testing.T) {
	tmpl := mustParse(t, "photo_{YYYY}_{original}{ext}")
	date := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	info := scanner.FileInfo{Name: "IMG_1234.jpg"}
	got := tmpl.Render(date, info, 0)
	want := "photo_2024_IMG_1234.jpg"
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestRender_NoExtPlaceholder(t *testing.T) {
	tmpl := mustParse(t, "{original}")
	info := scanner.FileInfo{Name: "IMG_1234.jpg"}
	got := tmpl.Render(time.Time{}, info, 0)
	want := "IMG_1234"
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestRender_DoubleExt(t *testing.T) {
	tmpl := mustParse(t, "{original}{ext}")
	info := scanner.FileInfo{Name: "archive.tar.gz"}
	got := tmpl.Render(time.Time{}, info, 0)
	want := "archive.tar.gz"
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestRender_UnicodeOriginal(t *testing.T) {
	tmpl := mustParse(t, "{original}{ext}")
	info := scanner.FileInfo{Name: "фотография_1.jpg"}
	got := tmpl.Render(time.Time{}, info, 0)
	want := "фотография_1.jpg"
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestRender_FileWithoutExt(t *testing.T) {
	tmpl := mustParse(t, "{original}{ext}")
	info := scanner.FileInfo{Name: "Makefile"}
	got := tmpl.Render(time.Time{}, info, 0)
	want := "Makefile"
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestRender_LongName(t *testing.T) {
	tmpl := mustParse(t, "{YYYY}-{MM}-{DD}_{HH}-{mm}-{SS}_{device}_{original}{ext}")
	date := time.Date(2024, 3, 15, 14, 30, 22, 0, time.UTC)
	info := scanner.FileInfo{Name: "very_long_original_filename_here.jpg", Device: "iPhone"}
	got := tmpl.Render(date, info, 0)
	want := "2024-03-15_14-30-22_iPhone_very_long_original_filename_here.jpg"
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestRender_OnlyExt(t *testing.T) {
	tmpl := mustParse(t, "{ext}")
	info := scanner.FileInfo{Name: ".gitignore"}
	got := tmpl.Render(time.Time{}, info, 0)
	// filepath.Ext(".gitignore") возвращает ".gitignore" (файл начинается с точки)
	want := ".gitignore"
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestRender_PathSeparatorInTemplate(t *testing.T) {
	tmpl := mustParse(t, "{YYYY}/{MM}/{original}{ext}")
	date := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	info := scanner.FileInfo{Name: "a.jpg"}
	got := tmpl.Render(date, info, 0)
	want := filepath.Join("2024", "03", "a.jpg")
	if got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}
