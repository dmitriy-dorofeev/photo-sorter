// Package copier отвечает за безопасное копирование файлов.
package copier

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// isWritableImage возвращает true для расширений, в которые можно записать EXIF через exiftool.
func isWritableImage(ext string) bool {
	ext = strings.ToLower(ext)
	switch ext {
	case ".jpg", ".jpeg", ".png", ".heic", ".heif":
		return true
	}
	return false
}

// writeExifDate записывает дату съёмки в тег DateTimeOriginal через exiftool.
// Формат даты для exiftool: "YYYY:MM:DD HH:mm:ss".
func writeExifDate(ctx context.Context, exifToolPath, targetPath string, date time.Time) error {
	if date.IsZero() {
		return fmt.Errorf("cannot write zero date to EXIF")
	}
	if exifToolPath == "" {
		exifToolPath = "exiftool"
	}
	dateStr := date.Format("2006:01:02 15:04:05")
	cmd := exec.CommandContext(ctx, exifToolPath,
		"-DateTimeOriginal="+dateStr,
		"-overwrite_original",
		"--", targetPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exiftool failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}
