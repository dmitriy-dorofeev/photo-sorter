// Package report генерирует итоговый отчёт о сортировке в текстовом или HTML-формате.
package report

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Data содержит всю информацию, необходимую для формирования отчёта.
type Data struct {
	Sources           []string
	Target            string
	FilesFound        int
	Copied            int
	Skipped           int
	Errors            int
	IntegrityFailures int
	ExifWrites        int
	ExifFailures      int
	BytesCopied       int64
	ErrorList         []error
	Duplicates        []DupGroup
	UnsortedFiles     []string
	FatalError        string
	Timestamp         time.Time
}

// DupGroup описывает группу дубликатов.
type DupGroup struct {
	Original   string
	Duplicates []string
	Strategy   string
}

// Generate создаёт файл отчёта в заданном формате в указанной директории.
// Возвращает путь к созданному файлу.
func Generate(targetDir, format string, data Data) (string, error) {
	if data.Timestamp.IsZero() {
		data.Timestamp = time.Now()
	}

	switch format {
	case "html":
		return generateHTML(targetDir, data)
	case "text":
		fallthrough
	default:
		return generateText(targetDir, data)
	}
}

func generateText(targetDir string, data Data) (string, error) {
	name := data.Timestamp.Format("2006-01-02_15-04-05") + "_photo-sorter.log"
	path := filepath.Join(targetDir, name)

	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	write := func(msg string) {
		_, _ = fmt.Fprintf(f, "[%s] %s\n", data.Timestamp.Format(time.RFC3339), msg)
	}

	write(fmt.Sprintf("Sources: %s", strings.Join(data.Sources, ", ")))
	write(fmt.Sprintf("Target: %s", data.Target))
	write(fmt.Sprintf("Files found: %d", data.FilesFound))
	write(fmt.Sprintf("Copied: %d", data.Copied))
	write(fmt.Sprintf("Skipped (duplicates): %d", data.Skipped))
	write(fmt.Sprintf("Errors: %d", data.Errors))
	if data.IntegrityFailures > 0 {
		write(fmt.Sprintf("Integrity failures: %d", data.IntegrityFailures))
	}
	if data.ExifWrites > 0 {
		write(fmt.Sprintf("EXIF writes: %d", data.ExifWrites))
	}
	if data.ExifFailures > 0 {
		write(fmt.Sprintf("EXIF failures: %d", data.ExifFailures))
	}
	write(fmt.Sprintf("Bytes copied: %d", data.BytesCopied))

	for _, e := range data.ErrorList {
		write(fmt.Sprintf("Error detail: %s", e.Error()))
	}

	for _, g := range data.Duplicates {
		for _, dup := range g.Duplicates {
			write(fmt.Sprintf("DUPLICATE: kept %s (strategy=%s), skipped %s (same hash)", g.Original, g.Strategy, dup))
		}
	}

	if data.FatalError != "" {
		write(fmt.Sprintf("Fatal error: %s", data.FatalError))
	}

	_ = f.Sync()
	return path, nil
}

func generateHTML(targetDir string, data Data) (string, error) {
	name := data.Timestamp.Format("2006-01-02_15-04-05") + "_photo-sorter.html"
	path := filepath.Join(targetDir, name)

	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := f.WriteString(buildHTML(data)); err != nil {
		return "", err
	}
	_ = f.Sync()
	return path, nil
}

func buildHTML(data Data) string {
	b := &strings.Builder{}

	fmt.Fprintf(b, `<!DOCTYPE html>
<html lang="ru">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Отчёт photo-sorter</title>
<style>
:root { --bg: #f5f7fa; --card: #ffffff; --text: #333333; --muted: #666666; --accent: #2563eb; --success: #16a34a; --warn: #ca8a04; --danger: #dc2626; }
* { box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; background: var(--bg); color: var(--text); margin: 0; padding: 24px; line-height: 1.5; }
.container { max-width: 960px; margin: 0 auto; }
header { margin-bottom: 24px; }
header h1 { margin: 0 0 8px; font-size: 1.6rem; }
header .meta { color: var(--muted); font-size: 0.9rem; }
.grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 16px; margin-bottom: 24px; }
.card { background: var(--card); border-radius: 12px; padding: 16px; box-shadow: 0 1px 3px rgba(0,0,0,0.08); }
.card .label { font-size: 0.85rem; color: var(--muted); margin-bottom: 4px; }
.card .value { font-size: 1.4rem; font-weight: 600; }
.card.success .value { color: var(--success); }
.card.warn .value { color: var(--warn); }
.card.danger .value { color: var(--danger); }
.section { background: var(--card); border-radius: 12px; padding: 20px; margin-bottom: 16px; box-shadow: 0 1px 3px rgba(0,0,0,0.08); }
.section h2 { margin: 0 0 12px; font-size: 1.1rem; border-bottom: 1px solid #e5e7eb; padding-bottom: 8px; }
table { width: 100%%; border-collapse: collapse; font-size: 0.95rem; }
th, td { text-align: left; padding: 8px 10px; border-bottom: 1px solid #f0f0f0; }
th { color: var(--muted); font-weight: 500; }
tr:last-child td { border-bottom: none; }
ul { margin: 0; padding-left: 20px; }
li { margin-bottom: 4px; }
.empty { color: var(--muted); font-style: italic; }
</style>
</head>
<body>
<div class="container">
`)

	// Header
	fmt.Fprintf(b, `<header>
<h1>📷 Отчёт photo-sorter</h1>
<div class="meta">%s</div>
</header>
`, data.Timestamp.Format("02.01.2006 15:04:05"))

	// Stats grid
	fmt.Fprint(b, `<div class="grid">
`)
	writeCard(b, "Найдено файлов", fmt.Sprintf("%d", data.FilesFound), "")
	writeCard(b, "Скопировано", fmt.Sprintf("%d", data.Copied), "success")
	writeCard(b, "Пропущено", fmt.Sprintf("%d", data.Skipped), "warn")
	if data.Errors > 0 {
		writeCard(b, "Ошибок", fmt.Sprintf("%d", data.Errors), "danger")
	} else {
		writeCard(b, "Ошибок", "0", "")
	}
	if data.IntegrityFailures > 0 {
		writeCard(b, "Целостность", fmt.Sprintf("%d", data.IntegrityFailures), "danger")
	}
	if data.ExifWrites > 0 {
		writeCard(b, "EXIF записан", fmt.Sprintf("%d", data.ExifWrites), "success")
	}
	if data.ExifFailures > 0 {
		writeCard(b, "Ошибок EXIF", fmt.Sprintf("%d", data.ExifFailures), "danger")
	}
	writeCard(b, "Байт скопировано", humanBytes(data.BytesCopied), "")
	fmt.Fprint(b, `</div>
`)

	// Paths
	fmt.Fprintf(b, `<div class="section">
<h2>🗂️ Пути</h2>
<table>
<tr><th>Источники</th><td>%s</td></tr>
<tr><th>Цель</th><td>%s</td></tr>
</table>
</div>
`, html.EscapeString(strings.Join(data.Sources, ", ")), html.EscapeString(data.Target))

	// Errors
	fmt.Fprint(b, `<div class="section">
<h2>⚠️ Ошибки</h2>
`)
	if len(data.ErrorList) == 0 && data.FatalError == "" {
		fmt.Fprint(b, `<p class="empty">Ошибок нет</p>
`)
	} else {
		fmt.Fprint(b, `<ul>
`)
		if data.FatalError != "" {
			fmt.Fprintf(b, `<li><strong>Fatal:</strong> %s</li>
`, html.EscapeString(data.FatalError))
		}
		for _, e := range data.ErrorList {
			fmt.Fprintf(b, `<li>%s</li>
`, html.EscapeString(e.Error()))
		}
		fmt.Fprint(b, `</ul>
`)
	}
	fmt.Fprint(b, `</div>
`)

	// Duplicates
	fmt.Fprint(b, `<div class="section">
<h2>🔁 Дубликаты</h2>
`)
	if len(data.Duplicates) == 0 {
		fmt.Fprint(b, `<p class="empty">Дубликатов не обнаружено</p>
`)
	} else {
		fmt.Fprint(b, `<table>
<tr><th>Оригинал</th><th>Дубликаты</th><th>Стратегия</th></tr>
`)
		for _, g := range data.Duplicates {
			dups := make([]string, len(g.Duplicates))
			for i, d := range g.Duplicates {
				dups[i] = html.EscapeString(filepath.Base(d))
			}
			fmt.Fprintf(b, `<tr><td>%s</td><td>%s</td><td>%s</td></tr>
`,
				html.EscapeString(filepath.Base(g.Original)),
				html.EscapeString(strings.Join(dups, ", ")),
				html.EscapeString(g.Strategy))
		}
		fmt.Fprint(b, `</table>
`)
	}
	fmt.Fprint(b, `</div>
`)

	// Unsorted
	fmt.Fprint(b, `<div class="section">
<h2>📁 Файлы без даты (unsorted)</h2>
`)
	if len(data.UnsortedFiles) == 0 {
		fmt.Fprint(b, `<p class="empty">Нет файлов без даты</p>
`)
	} else {
		fmt.Fprint(b, `<ul>
`)
		for _, u := range data.UnsortedFiles {
			fmt.Fprintf(b, `<li>%s</li>
`, html.EscapeString(filepath.Base(u)))
		}
		fmt.Fprint(b, `</ul>
`)
	}
	fmt.Fprint(b, `</div>
`)

	fmt.Fprint(b, `</div>
</body>
</html>
`)

	return b.String()
}

func writeCard(b *strings.Builder, label, value, class string) {
	cls := class
	if cls != "" {
		cls = " " + cls
	}
	fmt.Fprintf(b, `<div class="card%s"><div class="label">%s</div><div class="value">%s</div></div>
`, cls, html.EscapeString(label), html.EscapeString(value))
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for num := n / unit; num >= unit; num /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
