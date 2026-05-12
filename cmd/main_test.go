package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	cliBin    string
	dirtyBin  string
	buildOnce sync.Once
)

func buildTestBinaries() {
	buildOnce.Do(func() {
		tmpDir, err := os.MkdirTemp("", "photo-sorter-test-*")
		if err != nil {
			panic(fmt.Sprintf("failed to create temp dir: %v", err))
		}

		cliBin = filepath.Join(tmpDir, "photo-sorter")
		if out, err := exec.Command("go", "build", "-o", cliBin, ".").CombinedOutput(); err != nil {
			panic(fmt.Sprintf("failed to build binary: %v\n%s", err, out))
		}

		dirtyBin = filepath.Join(tmpDir, "photo-sorter-dirty")
		if out, err := exec.Command("go", "build", "-ldflags", "-X main.version=1.0.0-dirty", "-o", dirtyBin, ".").CombinedOutput(); err != nil {
			panic(fmt.Sprintf("failed to build dirty binary: %v\n%s", err, out))
		}
	})
}

func TestCLIVersion(t *testing.T) {
	buildTestBinaries()
	cmd := exec.Command(cliBin, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--version failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "dev") && !strings.Contains(output, "v") {
		t.Errorf("unexpected version output: %s", output)
	}
}

func TestCLICheckUpdateDev(t *testing.T) {
	buildTestBinaries()
	cmd := exec.Command(cliBin, "--check-update")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--check-update failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "dev") {
		t.Errorf("expected dev version message, got: %s", output)
	}
}

func TestCLIUpdateDev(t *testing.T) {
	buildTestBinaries()
	cmd := exec.Command(cliBin, "update")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected error for dev version update, got: %s", out)
	}
	output := string(out)
	if !strings.Contains(output, "dev") {
		t.Errorf("expected dev version message, got: %s", output)
	}
}

func TestCLICheckUpdateDirty(t *testing.T) {
	buildTestBinaries()
	cmd := exec.Command(dirtyBin, "--check-update")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected error for dirty version check-update, got: %s", out)
	}
	output := string(out)
	if !strings.Contains(output, "dirty") {
		t.Errorf("expected dirty version message, got: %s", output)
	}
}

func TestCLIUpdateDirty(t *testing.T) {
	buildTestBinaries()
	cmd := exec.Command(dirtyBin, "update")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected error for dirty version update, got: %s", out)
	}
	output := string(out)
	if !strings.Contains(output, "dirty") {
		t.Errorf("expected dirty version message, got: %s", output)
	}
}

func TestCLIHelp(t *testing.T) {
	buildTestBinaries()
	cmd := exec.Command(cliBin, "--help")
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
	if !strings.Contains(output, "--dup-strategy") {
		t.Error("help missing --dup-strategy flag")
	}
}

func TestCLIDryRun(t *testing.T) {
	buildTestBinaries()
	sourceDir := filepath.Join("..", "testdata", "e2e", "source", "2023")
	targetDir := t.TempDir()

	cmd := exec.Command(cliBin,
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
	var created []string
	filepath.WalkDir(targetDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		created = append(created, path)
		return nil
	})
	if len(created) > 0 {
		t.Errorf("dry-run created files: %v", created)
	}
}

func TestCLIJSON(t *testing.T) {
	buildTestBinaries()
	sourceDir := filepath.Join("..", "testdata", "e2e", "source", "2023")
	targetDir := t.TempDir()

	cmd := exec.Command(cliBin,
		"--source", sourceDir,
		"--target", targetDir,
		"--dry-run",
		"--format", "json",
	)
	out, err := cmd.Output()
	if err != nil {
		var stderr []byte
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr = exitErr.Stderr
		}
		t.Fatalf("CLI failed: %v\nstderr: %s\nstdout: %s", err, stderr, out)
	}

	var report jsonReport
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out)
	}
	if report.FilesFound == 0 {
		t.Errorf("expected files found > 0, got %d", report.FilesFound)
	}
	if report.Copied == 0 {
		t.Errorf("expected copied > 0, got %d", report.Copied)
	}
	if len(report.DuplicateGroups) == 0 {
		t.Error("expected duplicate groups in JSON")
	}
}

func TestCLIMultiSource(t *testing.T) {
	buildTestBinaries()
	sourceDir := filepath.Join("..", "testdata", "e2e", "source", "2023")
	targetDir := t.TempDir()

	cmd := exec.Command(cliBin,
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

func TestCLIValidation_DupStrategy(t *testing.T) {
	buildTestBinaries()
	sourceDir := filepath.Join("..", "testdata", "e2e", "source", "2023")
	targetDir := t.TempDir()

	cmd := exec.Command(cliBin,
		"--source", sourceDir,
		"--target", targetDir,
		"--dup-strategy", "foo",
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected error, got success:\n%s", out)
	}
	output := string(out)
	if !strings.Contains(output, "стратегия дедупликации должна быть одной из") {
		t.Errorf("expected error about invalid dup strategy, got:\n%s", output)
	}
}

func TestCLIValidation(t *testing.T) {
	buildTestBinaries()
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
			cmd := exec.Command(cliBin, tt.args...)
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
