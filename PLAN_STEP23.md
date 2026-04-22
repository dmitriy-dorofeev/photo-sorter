# План: Шаг 2.3 — Доработка `scanner`, `sorter`, `copier`

## 1. Цель и границы

Довести три core-модуля до production-ready состояния, чтобы они были готовы к интеграции с TUI (Шаг 3).

---

## 2. Архитектура изменений

### `internal/scanner/scanner.go`
- **Фильтрация по расширениям**: принимать whitelist расширений (`.jpg`, `.jpeg`, `.png`, `.heic`, `.heif`, `.mov`, `.mp4`, ...). Файлы с неизвестным расширением пропускаются.
- **Параллельный обход**: несколько `source` папок обрабатываются конкурентно через `golang.org/x/sync/errgroup`.
- **Интерфейс**: `New(sources []string, exts ...string) *Scanner`

### `internal/sorter/sorter.go`
- **`Entry.Skip`**: новое поле — файл помечен как дубликат и не подлежит копированию.
- **Обработка внутренних коллизий**: если два разных `source`-файла попадают в один `Target` (одна папка + одно имя), добавлять суффикс `_1`, `_2` и т.д.
- **Live Photos**: если `.MOV` не имеет даты, а `.HEIC`/`.HEIF` с тем же `basename` имеет — использовать дату из изображения, чтобы пара попала в одну папку.
- **Интерфейс**: `BuildTree(files []FileInfo, duplicates []deduper.Result, resolveDate func(FileInfo) (time.Time, bool)) []Entry`

### `internal/copier/copier.go`
- **`context.Context`**: поддержка отмены операции (`ctx.Done()`).
- **Прогресс**: callback `func(current, total int)` для обновления UI.
- **Статистика**: `Stats struct { Copied, Skipped, Errors int; BytesCopied int64 }`.
- **Проверка свободного места**: через `unix.Statfs` (macOS/Linux) перед началом копирования.
- **Обработка внешних коллизий**: если целевой файл уже существует:
  1. Сравнить по `xxhash` (через экспортированный `deduper.HashFile`).
  2. Хеши совпадают → пропускаем (`Skipped++`).
  3. Хеши разные → ищем свободное имя (`_1`, `_2`, …) и копируем.
- **Интерфейс**: `Copy(ctx context.Context, entries []Entry, progress func(current, total int)) (Stats, error)`

### `internal/deduper/hasher.go`
- Сделать `hashFile` экспортированным: `func HashFile(path string) (uint64, error)` — нужен для `copier`.

---

## 3. Пошаговая реализация

### Шаг 3.1. `scanner`
- Добавить зависимость `golang.org/x/sync`.
- Рефакторинг `Scanner`:
  - Поле `exts map[string]struct{}` (lowercase).
  - `New(sources []string, exts ...string)` — если `exts` пуст, принимать все файлы.
  - `Scan()` — для каждого `source` запускается `filepath.WalkDir` в горутине через `errgroup`.
  - Собирать результаты через защищённый мьютекс или канал.
- `scanner_test.go`: тесты на фильтрацию расширений, параллельный обход двух папок.

### Шаг 3.2. `sorter`
- Добавить `Skip bool` в `Entry`.
- Обновить `BuildTree`:
  - Принимать `duplicates []deduper.Result`.
  - Строить `map[string]struct{}` из путей дубликатов, помечать `Skip = true`.
  - Вести `map[string]int` счётчик коллизий имён внутри плана: `targetPath → count`.
  - При коллизии: `name.ext` → `name_1.ext`, `name_2.ext`.
  - Live Photos: предварительный проход по `files`, строим `map[basename]time.Time` для `.heic`/`.heif`.
  - Второй проход: для `.mov` без даты проверить `map`, если есть — использовать ту же дату.
- `sorter_test.go`: тесты на коллизии, Skip, Live Photos, unsorted.

### Шаг 3.3. `copier`
- Экспортировать `HashFile` из `deduper`.
- Рефакторинг `Copier.Copy`:
  - Сигнатура с `ctx` и `progress`.
  - Подсчёт требуемого места: сумма `Size` для `!Skip` записей.
  - Проверка `unix.Statfs(targetRoot)` → сравнение с требуемым местом.
  - Цикл по `entries`: проверка `ctx.Err()`, вызов `progress`, обработка коллизий.
- `copier_test.go`: тесты на dry-run, пропуск дублей, коллизию с переименованием.

### Шаг 3.4. Документация
- Обновить `ROADMAP.md`: 2.3 отметить ✅.
- Обновить `PLAN.md`: дописать детали реализации.

---

## 4. Критерии готовности

- [ ] `go test ./internal/scanner/ ./internal/sorter/ ./internal/copier/ ./internal/deduper/` — **все PASS**.
- [ ] `go build ./...` — **без ошибок**.
- [ ] `go mod tidy` — зависимости чистые.
- [ ] Коммит в Git с conventional message.

---

## 5. Зависимости

- `golang.org/x/sync` — `errgroup` для параллельного сканирования.
- `golang.org/x/sys/unix` — `Statfs` для проверки свободного места (уже есть indirect).
