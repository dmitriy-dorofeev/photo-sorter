package dateresolver

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// dateParser пытается распарсить дату из имени файла (без расширения).
type dateParser func(string) (time.Time, bool)

// Реестр парсеров. Порядок важен: более специфичные паттерны идут раньше.
var parsers = []dateParser{
	parseScreenshot,
	parseDateTimeWithDots,
	parseIMG,
	parseVID,
	parsePXL,
	parseSignal,
	parseIMGWA,
	parsePlainDateTime,
}

// parseFromFilename перебирает все известные паттерны и возвращает первую
// успешно распарсенную дату. Если ни один паттерн не подошёл — (_, false).
func parseFromFilename(name string) (time.Time, bool) {
	name = strings.TrimSuffix(name, filepath.Ext(name))
	for _, p := range parsers {
		if t, ok := p(name); ok {
			return t, true
		}
	}
	return time.Time{}, false
}

// Screenshot YYYY-MM-DD at HH.MM.SS
// Пример: "Screenshot 2024-03-15 at 14.30.22"
func parseScreenshot(name string) (time.Time, bool) {
	if !strings.HasPrefix(name, "Screenshot ") {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("Screenshot 2006-01-02 at 15.04.05", name, time.Local)
	return t, err == nil
}

// YYYY-MM-DD HH.MM.SS
// Пример: "2024-03-15 14.30.22"
func parseDateTimeWithDots(name string) (time.Time, bool) {
	const layout = "2006-01-02 15.04.05"
	if len(name) != len(layout) {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation(layout, name, time.Local)
	return t, err == nil
}

// IMG_YYYYMMDD_HHMMSS
// Пример: "IMG_20240315_143022"
func parseIMG(name string) (time.Time, bool) {
	if !strings.HasPrefix(name, "IMG_") {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("IMG_20060102_150405", name, time.Local)
	return t, err == nil
}

// VID_YYYYMMDD_HHMMSS
// Пример: "VID_20240315_143022"
func parseVID(name string) (time.Time, bool) {
	if !strings.HasPrefix(name, "VID_") {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("VID_20060102_150405", name, time.Local)
	return t, err == nil
}

// PXL_YYYYMMDD_HHMMSSmmm
// Пример: "PXL_20240315_143022123"
// Миллисекунды отбрасываем — парсим только базовую часть дата+время.
const (
	pxlPrefix    = "PXL_"
	pxlTotalLen  = len("PXL_20060102_150405000") // 22
	pxlPrefixLen = len(pxlPrefix)                // 4
	pxlCoreLen   = 15                            // "20060102_150405"
	pxlCoreEnd   = pxlPrefixLen + pxlCoreLen     // 19
)

func parsePXL(name string) (time.Time, bool) {
	if !strings.HasPrefix(name, pxlPrefix) {
		return time.Time{}, false
	}
	if len(name) != pxlTotalLen {
		return time.Time{}, false
	}
	core := name[pxlPrefixLen:pxlCoreEnd] // "20060102_150405"
	t, err := time.ParseInLocation("20060102_150405", core, time.Local)
	return t, err == nil
}

// signal-YYYY-MM-DD-HH-MM-SS
// Пример: "signal-2024-03-15-14-30-22"
func parseSignal(name string) (time.Time, bool) {
	if !strings.HasPrefix(name, "signal-") {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("signal-2006-01-02-15-04-05", name, time.Local)
	return t, err == nil
}

// imgWARe парсит формат IMG-YYYYMMDD-WA####.
// Компилируется один раз на уровне пакета.
var imgWARe = regexp.MustCompile(`^IMG-(\d{8})-WA\d+$`)

// IMG-YYYYMMDD-WA####
// Пример: "IMG-20240315-WA0001"
func parseIMGWA(name string) (time.Time, bool) {
	if !strings.HasPrefix(name, "IMG-") || !strings.Contains(name, "-WA") {
		return time.Time{}, false
	}
	matches := imgWARe.FindStringSubmatch(name)
	if len(matches) != 2 {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("20060102", matches[1], time.Local)
	return t, err == nil
}

// YYYYMMDD_HHMMSS
// Пример: "20240315_143022"
func parsePlainDateTime(name string) (time.Time, bool) {
	const layout = "20060102_150405"
	if len(name) != len(layout) {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation(layout, name, time.Local)
	return t, err == nil
}
