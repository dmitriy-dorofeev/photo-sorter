// Package facemodels отвечает за проверку наличия и скачивание
// ONNX-моделей для face-детекции и распознавания.
package facemodels

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const (
	detectionURL    = "https://github.com/opencv/opencv_zoo/raw/main/models/face_detection_yunet/face_detection_yunet_2023mar.onnx"
	recognitionURL  = "https://github.com/deepinsight/insightface/releases/download/v0.7/buffalo_s.zip"
	zipInternalName = "w600k_mbf.onnx"

	detectionFileName   = "face-detection.onnx"
	recognitionFileName = "face-recognition.onnx"
)

// EnsureModels проверяет наличие ONNX-моделей в modelDir и скачивает
// недостающие. Прогресс сообщений передаётся через progress callback.
func EnsureModels(modelDir string, progress func(msg string)) error {
	if err := os.MkdirAll(modelDir, 0750); err != nil {
		return fmt.Errorf("не удалось создать директорию моделей: %w", err)
	}

	detPath := filepath.Join(modelDir, detectionFileName)
	recPath := filepath.Join(modelDir, recognitionFileName)

	if _, err := os.Stat(detPath); os.IsNotExist(err) {
		if progress != nil {
			progress("Скачивание модели детекции лиц (YuNet)...")
		}
		if err := downloadFile(detectionURL, detPath); err != nil {
			return fmt.Errorf("не удалось скачать YuNet: %w", err)
		}
	}

	if _, err := os.Stat(recPath); os.IsNotExist(err) {
		if progress != nil {
			progress("Скачивание модели распознавания лиц (ArcFace)...")
		}
		if err := downloadAndExtractRecognition(modelDir); err != nil {
			return fmt.Errorf("не удалось скачать ArcFace: %w", err)
		}
	}

	return nil
}

// ModelsExist проверяет, что обе модели уже есть в modelDir.
func ModelsExist(modelDir string) bool {
	if _, err := os.Stat(filepath.Join(modelDir, detectionFileName)); os.IsNotExist(err) {
		return false
	}
	if _, err := os.Stat(filepath.Join(modelDir, recognitionFileName)); os.IsNotExist(err) {
		return false
	}
	return true
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func downloadAndExtractRecognition(modelDir string) error {
	// Скачиваем zip во временный файл.
	tmpZip, err := os.CreateTemp("", "buffalo_s_*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmpZip.Name())

	resp, err := http.Get(recognitionURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	if _, err := io.Copy(tmpZip, resp.Body); err != nil {
		return err
	}
	if err := tmpZip.Close(); err != nil {
		return err
	}

	// Распаковываем нужный файл из zip.
	zr, err := zip.OpenReader(tmpZip.Name())
	if err != nil {
		return err
	}
	defer zr.Close()

	var found *zip.File
	for _, f := range zr.File {
		if f.Name == zipInternalName {
			found = f
			break
		}
	}
	if found == nil {
		return fmt.Errorf("в архиве не найден %s", zipInternalName)
	}

	rc, err := found.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	destPath := filepath.Join(modelDir, recognitionFileName)
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}
