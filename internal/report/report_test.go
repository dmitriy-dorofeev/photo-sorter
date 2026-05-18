package report

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestGenerateText(t *testing.T) {
	tmp := t.TempDir()
	data := Data{
		Sources:    []string{"/src1", "/src2"},
		Target:     "/target",
		FilesFound: 10,
		Copied:     8,
		Skipped:    1,
		Errors:     1,
		Timestamp:  time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC),
	}

	path, err := Generate(tmp, "text", data)
	if err != nil {
		t.Fatalf("generate text: %v", err)
	}
	if !strings.HasSuffix(path, "_photo-sorter.log") {
		t.Errorf("expected .log suffix, got %s", path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(content), "Sources: /src1, /src2") {
		t.Errorf("missing sources in log")
	}
	if !strings.Contains(string(content), "Copied: 8") {
		t.Errorf("missing copied count in log")
	}
}

func TestGenerateHTML(t *testing.T) {
	tmp := t.TempDir()
	data := Data{
		Sources:       []string{"/src"},
		Target:        "/target",
		FilesFound:    5,
		Copied:        4,
		Skipped:       1,
		Errors:        0,
		BytesCopied:   1024 * 1024,
		UnsortedFiles: []string{"/src/unknown.jpg"},
		Duplicates: []DupGroup{
			{Original: "/src/a.jpg", Duplicates: []string{"/src/b.jpg"}, Strategy: "path"},
		},
		Timestamp: time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC),
	}

	path, err := Generate(tmp, "html", data)
	if err != nil {
		t.Fatalf("generate html: %v", err)
	}
	if !strings.HasSuffix(path, "_photo-sorter.html") {
		t.Errorf("expected .html suffix, got %s", path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read html: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "<!DOCTYPE html>") {
		t.Errorf("missing doctype")
	}
	if !strings.Contains(s, "Скопировано") {
		t.Errorf("missing 'Скопировано' in html")
	}
	if !strings.Contains(s, "unknown.jpg") {
		t.Errorf("missing unsorted file in html")
	}
	if !strings.Contains(s, "b.jpg") {
		t.Errorf("missing duplicate in html")
	}
	if !strings.Contains(s, "1.0 MB") {
		t.Errorf("missing human-readable bytes in html")
	}
}

func TestGenerateDefaultFormat(t *testing.T) {
	tmp := t.TempDir()
	data := Data{
		Sources:   []string{"/src"},
		Target:    "/target",
		Timestamp: time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC),
	}

	path, err := Generate(tmp, "unknown", data)
	if err != nil {
		t.Fatalf("generate default: %v", err)
	}
	if !strings.HasSuffix(path, "_photo-sorter.log") {
		t.Errorf("expected fallback to .log, got %s", path)
	}
}

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{1536 * 1024, "1.5 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
	}
	for _, c := range cases {
		got := humanBytes(c.n)
		if got != c.want {
			t.Errorf("humanBytes(%d) = %s, want %s", c.n, got, c.want)
		}
	}
}

func TestGenerateHTMLWithErrors(t *testing.T) {
	tmp := t.TempDir()
	data := Data{
		Sources:    []string{"/src"},
		Target:     "/target",
		Errors:     2,
		ErrorList:  []error{os.ErrNotExist, os.ErrPermission},
		FatalError: "disk full",
		Timestamp:  time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC),
	}

	path, err := Generate(tmp, "html", data)
	if err != nil {
		t.Fatalf("generate html with errors: %v", err)
	}

	content, _ := os.ReadFile(path)
	s := string(content)
	if !strings.Contains(s, "disk full") {
		t.Errorf("missing fatal error in html")
	}
	if !strings.Contains(s, "permission denied") {
		t.Errorf("missing error detail in html")
	}
}

func TestGenerateHTMLEscaping(t *testing.T) {
	tmp := t.TempDir()
	data := Data{
		Sources:   []string{"<script>alert(1)</script>"},
		Target:    "/target",
		Timestamp: time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC),
	}

	path, err := Generate(tmp, "html", data)
	if err != nil {
		t.Fatalf("generate html escaping: %v", err)
	}

	content, _ := os.ReadFile(path)
	s := string(content)
	if strings.Contains(s, "<script>alert(1)</script>") {
		t.Errorf("unescaped HTML content in report")
	}
	if !strings.Contains(s, "&lt;script&gt;") {
		t.Errorf("missing escaped HTML content")
	}
}
