package facemodels

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModelsExist(t *testing.T) {
	tmpDir := t.TempDir()

	if ModelsExist(tmpDir) {
		t.Fatal("ожидалось false для пустой директории")
	}

	// Создаём detection
	_ = os.WriteFile(filepath.Join(tmpDir, detectionFileName), []byte("fake"), 0644)
	if ModelsExist(tmpDir) {
		t.Fatal("ожидалось false при отсутствии recognition")
	}

	// Создаём recognition
	_ = os.WriteFile(filepath.Join(tmpDir, recognitionFileName), []byte("fake"), 0644)
	if !ModelsExist(tmpDir) {
		t.Fatal("ожидалось true при наличии обеих моделей")
	}
}
