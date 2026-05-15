package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"photo-sorter/internal/scanner"
)

func TestOpen_Close(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Проверяем, что файл создался.
	statePath := filepath.Join(dir, stateDirName, stateFileName)
	if _, err := os.Stat(statePath); err != nil {
		t.Errorf("state file not created: %v", err)
	}
}

func TestFilter(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	now := time.Now().Truncate(time.Second)

	files := []scanner.FileInfo{
		{Path: "/src/a.jpg", Size: 100, ModTime: now},
		{Path: "/src/b.jpg", Size: 200, ModTime: now.Add(time.Hour)},
	}

	// Первая фильтрация — state пуст, все файлы новые.
	toProcess, unchanged, err := s.Filter(files)
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	if len(toProcess) != 2 {
		t.Errorf("expected 2 toProcess, got %d", len(toProcess))
	}
	if len(unchanged) != 0 {
		t.Errorf("expected 0 unchanged, got %d", len(unchanged))
	}

	// Записываем state.
	records := []Record{
		{SourcePath: "/src/a.jpg", Size: 100, ModTime: now, TargetPath: "/dst/a.jpg"},
		{SourcePath: "/src/b.jpg", Size: 200, ModTime: now.Add(time.Hour), TargetPath: "/dst/b.jpg"},
	}
	if err := s.Update(records); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Повторная фильтрация — файлы неизменны.
	toProcess, unchanged, err = s.Filter(files)
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	if len(toProcess) != 0 {
		t.Errorf("expected 0 toProcess, got %d", len(toProcess))
	}
	if len(unchanged) != 2 {
		t.Errorf("expected 2 unchanged, got %d", len(unchanged))
	}

	// Изменяем один файл.
	files[0].Size = 150
	toProcess, unchanged, err = s.Filter(files)
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	if len(toProcess) != 1 || toProcess[0].Path != "/src/a.jpg" {
		t.Errorf("expected 1 toProcess (a.jpg), got %v", toProcess)
	}
	if len(unchanged) != 1 || unchanged[0].SourcePath != "/src/b.jpg" {
		t.Errorf("expected 1 unchanged (b.jpg), got %v", unchanged)
	}
}

func TestRecordsBySize(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	records := []Record{
		{SourcePath: "/src/a.jpg", Size: 100, FastHash: 1},
		{SourcePath: "/src/b.jpg", Size: 200, FastHash: 2},
		{SourcePath: "/src/c.jpg", Size: 100, FastHash: 3},
	}
	if err := s.Update(records); err != nil {
		t.Fatalf("Update: %v", err)
	}

	res, err := s.RecordsBySize(100)
	if err != nil {
		t.Fatalf("RecordsBySize: %v", err)
	}
	if len(res) != 2 {
		t.Errorf("expected 2 records with size 100, got %d", len(res))
	}

	res, err = s.RecordsBySize(999)
	if err != nil {
		t.Fatalf("RecordsBySize: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("expected 0 records with size 999, got %d", len(res))
	}
}

func TestCleanup(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	records := []Record{
		{SourcePath: "/src/a.jpg", Size: 100},
		{SourcePath: "/src/b.jpg", Size: 200},
		{SourcePath: "/src/c.jpg", Size: 300},
	}
	if err := s.Update(records); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Оставляем только a и c.
	if err := s.Cleanup([]string{"/src/a.jpg", "/src/c.jpg"}); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	res, err := s.RecordsBySize(200)
	if err != nil {
		t.Fatalf("RecordsBySize: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("expected b.jpg removed, got %d records", len(res))
	}

	all := 0
	for _, size := range []int64{100, 200, 300} {
		r, _ := s.RecordsBySize(size)
		all += len(r)
	}
	if all != 2 {
		t.Errorf("expected 2 total records after cleanup, got %d", all)
	}
}

func TestReset(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.Update([]Record{{SourcePath: "/src/a.jpg", Size: 100}}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := Reset(dir); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	statePath := filepath.Join(dir, stateDirName, stateFileName)
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Error("expected state file to be removed")
	}
}

func TestReset_NonExistent(t *testing.T) {
	dir := t.TempDir()
	if err := Reset(dir); err != nil {
		t.Fatalf("Reset on empty dir: %v", err)
	}
}
