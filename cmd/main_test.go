package main

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIHelp(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// flag --help returns exit code 0 in Go, but let's be safe
		if cmd.ProcessState.ExitCode() != 0 {
			t.Fatalf("--help failed: %v\n%s", err, out)
		}
	}
	output := string(out)
	if !strings.Contains(output, "photo-sorter") {
		t.Error("help missing app name")
	}
	if !strings.Contains(output, "--source") {
		t.Error("help missing --source flag")
	}
	if !strings.Contains(output, "--target") {
		t.Error("help missing --target flag")
	}
	if !strings.Contains(output, "--dry-run") {
		t.Error("help missing --dry-run flag")
	}
	if !strings.Contains(output, "--format") {
		t.Error("help missing --format flag")
	}
}

func TestCLIDryRun(t *testing.T) {
	sourceDir := filepath.Join("..", "testdata", "e2e", "source", "2023")
	targetDir := t.TempDir()

	cmd := exec.Command("go", "run", ".",
		"--source", sourceDir,
		"--target", targetDir,
		"--dry-run",
		"--format", "text",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "Найдено файлов:") {
		t.Errorf("missing 'Найдено файлов' in output:\n%s", output)
	}
	if !strings.Contains(output, "Скопировано:") {
		t.Errorf("missing 'Скопировано' in output:\n%s", output)
	}

	// Verify no files were actually created
	entries, _ := filepath.Glob(filepath.Join(targetDir, "*"))
	if len(entries) > 0 {
		t.Errorf("dry-run created files: %v", entries)
	}
}

func TestCLIJSON(t *testing.T) {
	sourceDir := filepath.Join("..", "testdata", "e2e", "source", "2023")
	targetDir := t.TempDir()

	cmd := exec.Command("go", "run", ".",
		"--source", sourceDir,
		"--target", targetDir,
		"--dry-run",
		"--format", "json",
	)
	out, err := cmd.Output()
	if err != nil {
		var stderr []byte
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = exitErr.Stderr
		}
		t.Fatalf("CLI failed: %v\nstderr: %s\nstdout: %s", err, stderr, out)
	}

	var report jsonReport
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out)
	}
	if report.FilesFound != 12 {
		t.Errorf("expected 12 files, got %d", report.FilesFound)
	}
	if report.Copied == 0 {
		t.Errorf("expected copied > 0, got %d", report.Copied)
	}
	if len(report.DuplicateGroups) == 0 {
		t.Error("expected duplicate groups in JSON")
	}
}

func TestCLIMultiSource(t *testing.T) {
	sourceDir := filepath.Join("..", "testdata", "e2e", "source", "2023")
	targetDir := t.TempDir()

	cmd := exec.Command("go", "run", ".",
		"--source", sourceDir,
		"--source", sourceDir,
		"--target", targetDir,
		"--dry-run",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "Найдено файлов:") {
		t.Errorf("missing stats in output:\n%s", output)
	}
}

func TestCLIValidation(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		errMsg string
	}{
		{
			name:   "no source",
			args:   []string{"--target", "/tmp/out"},
			errMsg: "укажите хотя бы одну исходную папку",
		},
		{
			name:   "no target",
			args:   []string{"--source", "/tmp/in"},
			errMsg: "укажите целевую папку",
		},
		{
			name:   "invalid format",
			args:   []string{"--source", "/tmp/in", "--target", "/tmp/out", "--format", "xml"},
			errMsg: "формат должен быть",
		},
		{
			name:   "missing source dir",
			args:   []string{"--source", "/nonexistent/path", "--target", "/tmp/out"},
			errMsg: "не существует",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"run", "."}, tt.args...)
			cmd := exec.Command("go", args...)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("expected error, got success:\n%s", out)
			}
			output := string(out)
			if !strings.Contains(output, tt.errMsg) {
				t.Errorf("expected error containing %q, got:\n%s", tt.errMsg, output)
			}
		})
	}
}
