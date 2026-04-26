package logger

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestNew_Error(t *testing.T) {
	// Путь к несуществующей директории
	_, err := New("/nonexistent/dir/file.log")
	if err == nil {
		t.Error("expected error for nonexistent dir")
	}

	// Путь — существующая директория, а не файл
	dir := t.TempDir()
	_, err = New(dir)
	if err == nil {
		t.Error("expected error when path is a directory")
	}
}

func TestLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.log")
	l, err := New(path)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	if err := l.Log("first line"); err != nil {
		t.Fatalf("log first: %v", err)
	}
	if err := l.Log("second line"); err != nil {
		t.Fatalf("log second: %v", err)
	}

	if err := l.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "first line") {
		t.Errorf("missing first line in log: %s", content)
	}
	if !strings.Contains(content, "second line") {
		t.Errorf("missing second line in log: %s", content)
	}
	// Каждая запись на отдельной строке
	if strings.Count(content, "\n") != 2 {
		t.Errorf("expected 2 lines, got %d", strings.Count(content, "\n"))
	}
}

func TestLog_Concurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "race.log")
	l, err := New(path)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	defer l.Close()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := l.Log("msg"); err != nil {
				t.Errorf("log %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if strings.Count(string(data), "\n") != 100 {
		t.Errorf("expected 100 lines, got %d", strings.Count(string(data), "\n"))
	}
}

func TestLog_AfterClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "closed.log")
	l, err := New(path)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := l.Log("after close"); err == nil {
		t.Error("expected error writing to closed log")
	}
}
