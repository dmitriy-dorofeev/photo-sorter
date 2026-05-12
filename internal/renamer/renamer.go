// Package renamer отвечает за генерацию имён файлов по пользовательскому шаблону.
package renamer

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"photo-sorter/internal/scanner"
)

// Template — валидированный шаблон имени файла.
type Template struct {
	parts []part
}

type part struct {
	isPlaceholder bool
	text          string // статический текст, если isPlaceholder == false
	ph            string // имя плейсхолдера, если isPlaceholder == true
	seqWidth      int    // ширина для seq (0 — без ведущих нулей)
}

// Parse проверяет шаблон и возвращает готовый к рендерингу объект.
func Parse(tmpl string) (*Template, error) {
	if tmpl == "" {
		return nil, fmt.Errorf("шаблон имени файла не может быть пустым")
	}

	var parts []part
	i := 0
	for i < len(tmpl) {
		openIdx := strings.Index(tmpl[i:], "{")
		if openIdx == -1 {
			parts = append(parts, part{text: tmpl[i:]})
			break
		}
		openIdx += i

		if openIdx > i {
			parts = append(parts, part{text: tmpl[i:openIdx]})
		}

		closeIdx := strings.Index(tmpl[openIdx:], "}")
		if closeIdx == -1 {
			return nil, fmt.Errorf("не закрыт плейсхолдер в позиции %d", openIdx)
		}
		closeIdx += openIdx

		ph := tmpl[openIdx+1 : closeIdx]
		p, err := parsePlaceholder(ph)
		if err != nil {
			return nil, err
		}
		parts = append(parts, p)
		i = closeIdx + 1
	}

	return &Template{parts: parts}, nil
}

func parsePlaceholder(ph string) (part, error) {
	if strings.HasPrefix(ph, "seq:") {
		widthStr := ph[4:]
		w, err := strconv.Atoi(widthStr)
		if err != nil || w < 1 {
			return part{}, fmt.Errorf("невалидная ширина seq: %s", widthStr)
		}
		return part{isPlaceholder: true, ph: "seq", seqWidth: w}, nil
	}

	switch ph {
	case "YYYY", "YY", "MM", "DD", "HH", "mm", "SS", "original", "ext", "device", "seq":
		return part{isPlaceholder: true, ph: ph}, nil
	default:
		return part{}, fmt.Errorf("неизвестный плейсхолдер: {%s}", ph)
	}
}

// Render формирует имя файла по шаблону.
//
//	date — дата съёмки (time.Time{} для unsorted).
//	info — метаданные файла.
//	seq  — порядковый номер (для плейсхолдера {seq}).
func (t *Template) Render(date time.Time, info scanner.FileInfo, seq int) string {
	var b strings.Builder

	for _, p := range t.parts {
		if !p.isPlaceholder {
			b.WriteString(p.text)
			continue
		}

		switch p.ph {
		case "YYYY":
			if date.IsZero() {
				b.WriteString("0000")
			} else {
				b.WriteString(fmt.Sprintf("%04d", date.Year()))
			}
		case "YY":
			if date.IsZero() {
				b.WriteString("00")
			} else {
				b.WriteString(fmt.Sprintf("%02d", date.Year()%100))
			}
		case "MM":
			if date.IsZero() {
				b.WriteString("00")
			} else {
				b.WriteString(fmt.Sprintf("%02d", date.Month()))
			}
		case "DD":
			if date.IsZero() {
				b.WriteString("00")
			} else {
				b.WriteString(fmt.Sprintf("%02d", date.Day()))
			}
		case "HH":
			if date.IsZero() {
				b.WriteString("00")
			} else {
				b.WriteString(fmt.Sprintf("%02d", date.Hour()))
			}
		case "mm":
			if date.IsZero() {
				b.WriteString("00")
			} else {
				b.WriteString(fmt.Sprintf("%02d", date.Minute()))
			}
		case "SS":
			if date.IsZero() {
				b.WriteString("00")
			} else {
				b.WriteString(fmt.Sprintf("%02d", date.Second()))
			}
		case "original":
			ext := filepath.Ext(info.Name)
			b.WriteString(strings.TrimSuffix(info.Name, ext))
		case "ext":
			b.WriteString(filepath.Ext(info.Name))
		case "device":
			b.WriteString(info.Device)
		case "seq":
			if p.seqWidth > 0 {
				b.WriteString(fmt.Sprintf("%0*d", p.seqWidth, seq))
			} else {
				b.WriteString(strconv.Itoa(seq))
			}
		}
	}

	return b.String()
}
