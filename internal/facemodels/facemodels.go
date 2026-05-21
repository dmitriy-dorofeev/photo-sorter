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

// progressReader оборачивает io.Reader и вызывает onProg при изменении прогресса.
type progressReader struct {
	r       io.Reader
	total   int64
	current int64
	onProg  func(current, total int64)
	lastPct int
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	pr.current += int64(n)
	if pr.onProg != nil {
		if pr.total > 0 {
			pct := int(pr.current * 100 / pr.total)
			if pct != pr.lastPct {
				pr.lastPct = pct
				pr.onProg(pr.current, pr.total)
			}
		} else {
			const chunk = 256 * 1024
			if pr.current >= int64(pr.lastPct)*chunk {
				pr.lastPct = int(pr.current / chunk)
				pr.onProg(pr.current, pr.total)
			}
		}
	}
	return n, err
}

// EnsureModels проверяет наличие ONNX-моделей в modelDir и скачивает
// недостающие. Прогресс передаётся через progress callback в байтах
// (current, total). Если total == 0, размер заранее неизвестен.
func EnsureModels(modelDir string, progress func(current, total int64)) error {
	if err := os.MkdirAll(modelDir, 0750); err != nil {
		return fmt.Errorf("не удалось создать директорию моделей: %w", err)
	}

	detPath := filepath.Join(modelDir, detectionFileName)
	recPath := filepath.Join(modelDir, recognitionFileName)

	if _, err := os.Stat(detPath); os.IsNotExist(err) {
		if progress != nil {
			progress(0, 0)
		}
		if err := downloadFile(detectionURL, detPath, progress); err != nil {
			return fmt.Errorf("не удалось скачать YuNet: %w", err)
		}
	}

	if _, err := os.Stat(recPath); os.IsNotExist(err) {
		if progress != nil {
			progress(0, 0)
		}
		if err := downloadAndExtractRecognition(modelDir, progress); err != nil {
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
func downloadFile(url, dest string, progress func(current, total int64)) error {
	client := &http.Client{Timeout: downloadTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var total int64
	if resp.ContentLength > 0 {
		total = resp.ContentLength
	}

	cleanDest := filepath.Clean(dest)
	//nolint:gosec // G304: путь формируется из констант и проверенного modelDir.
	out, err := os.Create(cleanDest)
	if err != nil {
		return err
	}
	defer out.Close()

	var src io.Reader = resp.Body
	if progress != nil {
		src = &progressReader{r: resp.Body, total: total, onProg: progress}
	}

	_, err = io.Copy(out, io.LimitReader(src, maxModelSize))
	return err
}

//nolint:gosec // G107: URL — внутренняя константа пакета.
func downloadAndExtractRecognition(modelDir string, progress func(current, total int64)) error {
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

	var total int64
	if resp.ContentLength > 0 {
		total = resp.ContentLength
	}

	var src io.Reader = resp.Body
	if progress != nil {
		src = &progressReader{r: resp.Body, total: total, onProg: progress}
	}

	if _, err := io.Copy(tmpZip, io.LimitReader(src, maxModelSize)); err != nil {
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

	_, err = io.Copy(out, io.LimitReader(rc, maxModelSize))
	return err
}
