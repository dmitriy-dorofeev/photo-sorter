// Package depcheck выполняет проверку внешних зависимостей проекта
// (exiftool, ONNX Runtime) и формирует инструкции по их установке.
package depcheck

import (
	"fmt"
	"runtime"
	"strings"

	"photo-sorter/internal/dateresolver"
	"photo-sorter/internal/onnxhelper"
)

// Status описывает состояние зависимости.
type Status int

const (
	StatusOK Status = iota
	StatusMissing
)

// Result — результат проверки одной зависимости.
type Result struct {
	Name        string
	Description string
	Optional    bool
	Status      Status
	Details     string
	InstallHint map[string]string // ключ: GOOS (darwin, linux)
}

// Results — набор результатов проверки.
type Results []Result

// HasMissingRequired возвращает true, если хотя бы одна обязательная зависимость отсутствует.
func (rr Results) HasMissingRequired() bool {
	for _, r := range rr {
		if !r.Optional && r.Status == StatusMissing {
			return true
		}
	}
	return false
}

// FilterMissing возвращает только отсутствующие зависимости.
func (rr Results) FilterMissing() Results {
	var out Results
	for _, r := range rr {
		if r.Status == StatusMissing {
			out = append(out, r)
		}
	}
	return out
}

// CheckAll выполняет проверку всех внешних зависимостей проекта.
func CheckAll() Results {
	return Results{
		checkExifTool(),
		checkONNXRuntime(),
	}
}

func checkExifTool() Result {
	path, ok := dateresolver.FindExifTool()
	r := Result{
		Name:        "exiftool",
		Description: "Чтение видео-метаданных и запись EXIF",
		Optional:    true,
		InstallHint: map[string]string{
			"darwin": "brew install exiftool",
			"linux":  "sudo apt install libimage-exiftool-perl    # Debian/Ubuntu\nsudo pacman -S perl-image-exiftool    # Arch",
		},
	}
	if ok {
		r.Status = StatusOK
		r.Details = path
	} else {
		r.Status = StatusMissing
		r.Details = "не найден в PATH"
	}
	return r
}

func checkONNXRuntime() Result {
	r := Result{
		Name:        "ONNX Runtime",
		Description: "Face-детекция и распознавание (face-режим)",
		Optional:    true,
		InstallHint: map[string]string{
			"darwin": "brew install onnxruntime",
			"linux":  "sudo apt install libonnxruntime-dev    # Debian/Ubuntu\nили скачайте .tgz с https://github.com/microsoft/onnxruntime/releases",
		},
	}
	if _, err := onnxhelper.Runtime(); err == nil {
		r.Status = StatusOK
		r.Details = "доступен"
	} else {
		r.Status = StatusMissing
		r.Details = err.Error()
	}
	return r
}

// RenderText возвращает текстовое представление результатов для CLI.
func (rr Results) RenderText() string {
	var b strings.Builder
	b.WriteString("Зависимость        Статус                Детали\n")
	b.WriteString(strings.Repeat("─", 60) + "\n")
	for _, r := range rr {
		status := "✅ OK"
		if r.Status == StatusMissing {
			if r.Optional {
				status = "⚠️  ОТСУТСТВУЕТ (опц.)"
			} else {
				status = "❌ ОТСУТСТВУЕТ"
			}
		}
		b.WriteString(fmt.Sprintf("%-18s %-22s %s\n", r.Name, status, r.Details))
	}

	missing := rr.FilterMissing()
	if len(missing) > 0 {
		b.WriteString("\nУстановка:\n")
		for _, r := range missing {
			if hint, ok := r.InstallHint[runtime.GOOS]; ok && hint != "" {
				b.WriteString(fmt.Sprintf("  %s:\n    %s\n", r.Name, strings.ReplaceAll(hint, "\n", "\n    ")))
			}
		}
	}
	return b.String()
}
