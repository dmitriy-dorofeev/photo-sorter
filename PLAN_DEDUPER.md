# План: `internal/deduper` — движок дедупликации

## 1. Цель и границы

Реализовать двухуровневый детектор дубликатов для photo-sorter:
- **Быстрый уровень**: группировка файлов по размеру.
- **Точный уровень**: вычисление `xxhash` внутри групп одинакового размера.
- **Edge case**: пары Live Photos (`.HEIC` + `.MOV` с одним basename) не считаются дубликатами друг друга.
- **Выход**: `[]Result`, где каждый элемент содержит оригинал и список его дубликатов.

## 2. Архитектура

```
internal/deduper/
├── hasher.go           # потоковое хеширование файла через xxhash
├── deduper.go          # группировка по размеру → хешу, фильтрация Live Photos
└── deduper_test.go     # unit-тесты + тестовые данные
```

### Интерфейсы

- `func hashFile(path string) (uint64, error)` — возвращает xxhash файла.
- `func (d *Deduper) FindDuplicates() []Result` — обновлённая реализация:
  1. `map[int64][]FileInfo` — группировка по размеру.
  2. Для групп с `len > 1` — параллельное вычисление хеша.
  3. `map[uint64][]FileInfo` — группировка по хешу внутри размера.
  4. Фильтрация Live Photos: если два файла имеют одинаковый basename (без расширения) и расширения `.heic`/`.mov` (или `.heif`/`.mov`) — исключить их из дубликатов.
  5. Первый файл в группе — `Original`, остальные — `Duplicates`.

## 3. Пошаговая реализация

### Шаг 3.1. `hasher.go` — хеширование файла

- Добавить зависимость: `go get github.com/cespare/xxhash/v2`.
- Реализовать `hashFile`:
  - Открыть файл через `os.Open`.
  - Читать блоками через `bufio.Reader` (например, 64 KiB).
  - Обновлять `xxhash.Digest`.
  - Вернуть `Sum64()`.
- Обработка ошибок: вернуть `(_, error)` при любой проблеме с файлом.

### Шаг 3.2. `deduper.go` — логика поиска дубликатов

- Заменить заглушку `FindDuplicates()`:
  - **Группировка по размеру**: пройти по `d.files`, заполнить `map[int64][]scanner.FileInfo`.
  - **Фильтрация**: оставить только группы с `len >= 2`.
  - **Хеширование**: для каждого файла в группе вызвать `hashFile`.
  - **Группировка по хешу**: `map[uint64][]scanner.FileInfo` внутри каждой размерной группы.
  - **Live Photos check**: перед формированием `Result` проверить, не являются ли файлы парой Live Photos.
    - Условие: `strings.EqualFold(stripExt(a.Name), stripExt(b.Name))` и один из них `.heic`/`.heif`, а другой `.mov`.
    - Если вся группа — только Live Photos пара, пропустить её.
    - Если в группе больше файлов (например, 3+), а два из них — Live Photos, остальные всё равно могут быть дубликатами.
  - **Формирование Result**: первый файл — `Original`, остальные — `Duplicates`.

### Шаг 3.3. Тестовые данные

Создать файлы в `testdata/deduper/`:

| Файл | Размер | Содержимое | Назначение |
|------|--------|------------|------------|
| `dup_original.bin` | 100 B | случайные байты | оригинал |
| `dup_copy.bin` | 100 B | **точная копия** `dup_original.bin` | дубликат |
| `same_size_a.bin` | 100 B | другие случайные байты | тот же размер, другой хеш |
| `same_size_b.bin` | 100 B | ещё другие байты | тот же размер, другой хеш |
| `live_photo.heic` | 50 B | байты A | Live Photos пара |
| `live_photo.mov` | 50 B | байты B | Live Photos пара |
| `single.bin` | 200 B | уникальные байты | одиночный файл (не дубль) |

> Создадим через Shell: `dd if=/dev/urandom of=... bs=1 count=...`.

### Шаг 3.4. `deduper_test.go` — unit-тесты

Табличные тесты:

1. `TestHashFile_Stability` — два запуска на одном файле дают одинаковый хеш.
2. `TestFindDuplicates_EmptyInput` — `nil` файлов → `nil` результат.
3. `TestFindDuplicates_NoDuplicates` — все файлы разного размера → `nil`.
4. `TestFindDuplicates_ExactDuplicates` — `dup_original.bin` + `dup_copy.bin` → 1 Result с 1 Duplicate.
5. `TestFindDuplicates_SameSizeDifferentContent` — `same_size_a.bin` + `same_size_b.bin` → `nil` (разные хеши).
6. `TestFindDuplicates_LivePhotos` — `live_photo.heic` + `live_photo.mov` → `nil` (не дубли).
7. `TestFindDuplicates_LivePhotosWithDuplicate` — 3 файла: `.heic`, `.mov`, и точная копия `.heic` → Result, где оригинал `.heic`, дубликат — копия `.heic`, `.mov` не попадает в дубликаты.

### Шаг 3.5. Документация

- Обновить `ROADMAP.md`: отметить 2.2 ✅ и добавить ссылки на файлы.
- Обновить `PLAN.md`: дописать детали реализации `deduper` в раздел "Неделя 2".

## 4. Критерии готовности

- [ ] `go test ./internal/deduper/` — **все PASS**.
- [ ] `go build ./...` — **без ошибок**.
- [ ] `go mod tidy` — зависимости чистые.
- [ ] Все edge cases покрыты тестами.
- [ ] Коммит в Git с conventional message.

## 5. Зависимости

- `github.com/cespare/xxhash/v2` — добавить через `go get`.
