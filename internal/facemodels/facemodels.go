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
	"time"
)

const (
	detectionURL    = "https://github.com/opencv/opencv_zoo/raw/main/models/face_detection_yunet/face_detection_yunet_2023mar.onnx"
	recognitionURL  = "https://github.com/deepinsight/insightface/releases/download/v0.7/buffalo_s.zip"
	zipInternalName = "w600k_mbf.onnx"

	detectionFileName   = "face-detection.onnx"
	recognitionFileName = "face-recognition.onnx"

	// Лимит на размер скачиваемой/распаковываемой модели (200 MB).
	maxModelSize = 200 * 1024 * 1024

	downloadTimeout = 5 * time.Minute
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

//nolint:gosec // G107: URL — внутренние константы пакета, не пользовательский ввод.
func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: downloadTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	cleanDest := filepath.Clean(dest)
	//nolint:gosec // G304: путь формируется из констант и проверенного modelDir.
	out, err := os.Create(cleanDest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.CopyN(out, resp.Body, maxModelSize)
	return err
}

//nolint:gosec // G107: URL — внутренняя константа пакета.
func downloadAndExtractRecognition(modelDir string) error {
	// Скачиваем zip во временный файл.
	tmpZip, err := os.CreateTemp("", "buffalo_s_*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmpZip.Name())

	client := &http.Client{Timeout: downloadTimeout}
	resp, err := client.Get(recognitionURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	if _, err := io.CopyN(tmpZip, resp.Body, maxModelSize); err != nil {
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

	cleanDest := filepath.Join(filepath.Clean(modelDir), recognitionFileName)
	// #nosec G304 — путь формируется из констант и проверенного modelDir.
	out, err := os.Create(cleanDest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.CopyN(out, rc, maxModelSize)
	return err
}
