package updater

import (
	"testing"
)

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"dev", "dev"},
		{"1.0.0", "v1.0.0"},
		{"v1.0.0", "v1.0.0"},
		{"1.2.3-dirty", "v1.2.3-dirty"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeVersion(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeVersion(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCheckVersionDirty(t *testing.T) {
	res := CheckVersion("1.0.0-dirty")
	if !res.IsDirty {
		t.Error("expected IsDirty to be true")
	}
	if res.IsDev {
		t.Error("expected IsDev to be false")
	}
	if res.HasUpdate {
		t.Error("expected HasUpdate to be false for dirty version")
	}
}

func TestCheckVersionDev(t *testing.T) {
	res := CheckVersion("dev")
	if res.IsDirty {
		t.Error("expected IsDirty to be false")
	}
	if !res.IsDev {
		t.Error("expected IsDev to be true")
	}
	if res.HasUpdate {
		t.Error("expected HasUpdate to be false for dev version")
	}
}

func TestCheckVersionInvalid(t *testing.T) {
	res := CheckVersion("some-random-string")
	if res.IsDirty {
		t.Error("expected IsDirty to be false")
	}
	if !res.IsDev {
		t.Error("expected IsDev to be true for invalid version")
	}
}
