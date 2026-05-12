package collision

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolve_Counter(t *testing.T) {
	target := filepath.Join("/out", "2024", "01", "01", "photo.jpg")

	got := Resolve(target, StrategyCounter, "/src/photo.jpg", 0)
	want := filepath.Join("/out", "2024", "01", "01", "photo_1.jpg")
	if got != want {
		t.Errorf("index 0 = %q, want %q", got, want)
	}

	got = Resolve(target, StrategyCounter, "/src/photo.jpg", 1)
	want = filepath.Join("/out", "2024", "01", "01", "photo_2.jpg")
	if got != want {
		t.Errorf("index 1 = %q, want %q", got, want)
	}
}

func TestResolve_Hash(t *testing.T) {
	target := filepath.Join("/out", "2024", "01", "01", "photo.jpg")

	got := Resolve(target, StrategyHash, "/src/photo.jpg", 0)
	if got == target {
		t.Error("hash resolve should change the name")
	}
	if strings.HasSuffix(got, "_1.jpg") {
		t.Error("first hash attempt should not have _1 fallback")
	}

	got = Resolve(target, StrategyHash, "/src/photo.jpg", 1)
	if !strings.HasSuffix(got, "_1.jpg") {
		t.Errorf("expected _1 fallback, got %q", got)
	}
}

func TestResolve_HashDifferentPaths(t *testing.T) {
	target := filepath.Join("/out", "2024", "01", "01", "photo.jpg")

	a := Resolve(target, StrategyHash, "/src/a.jpg", 0)
	b := Resolve(target, StrategyHash, "/src/b.jpg", 0)
	if a == b {
		t.Error("different paths should produce different hash suffixes")
	}
}

func TestResolve_HashSamePathDeterministic(t *testing.T) {
	target := filepath.Join("/out", "2024", "01", "01", "photo.jpg")

	a := Resolve(target, StrategyHash, "/src/photo.jpg", 0)
	b := Resolve(target, StrategyHash, "/src/photo.jpg", 0)
	if a != b {
		t.Errorf("same path should produce same hash suffix: %q vs %q", a, b)
	}
}
