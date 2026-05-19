package spotlight

import (
	"runtime"
	"testing"
	"time"
)

func TestAvailable(t *testing.T) {
	got := Available()
	if runtime.GOOS == "darwin" {
		if !got {
			t.Error("expected Available() = true on darwin")
		}
	} else {
		if got {
			t.Error("expected Available() = false on non-darwin")
		}
	}
}

func TestWriteTags_NoOpOnUnsupported(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("skipping on darwin")
	}
	if err := WriteTags("/tmp/fake", time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Errorf("expected nil on unsupported platform, got %v", err)
	}
}
