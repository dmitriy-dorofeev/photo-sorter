// Package copier отвечает за безопасное копирование файлов.
package copier

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"photo-sorter/internal/sorter"
)

// Copier выполняет копирование файлов.
type Copier struct {
	dryRun bool
}

// New создаёт новый Copier.
func New(dryRun bool) *Copier {
	return &Copier{dryRun: dryRun}
}

// Copy выполняет копирование по плану сортировки.
// TODO: добавить прогресс, отмену, обработку коллизий имён.
func (c *Copier) Copy(entries []sorter.Entry) error {
	for _, e := range entries {
		if c.dryRun {
			fmt.Printf("[dry-run] %s -> %s\n", e.Source.Path, e.Target)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(e.Target), 0755); err != nil {
			return err
		}

		if err := copyFile(e.Source.Path, e.Target); err != nil {
			return err
		}
	}
	return nil
}

// copyFile копирует содержимое одного файла в другой.
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	return destFile.Sync()
}
