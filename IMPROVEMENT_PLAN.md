# IMPROVEMENT_PLAN.md

> Ревью проекта **photo-sorter** от Senior Go Developer.
> Статус: полный аудит кодовой базы (~5K строк Go).
> Цель: устранить критические баги, закрыть security-дыры, повысить производительность и надёжность.

---

## Содержание

1. [Безопасность](#1-безопасность)
2. [Производительность](#2-производительность)
3. [Архитектура и связанность](#3-архитектура-и-связанность)
4. [Обработка ошибок и надёжность](#4-обработка-ошибок-и-надёжность)
5. [TUI и пользовательский опыт](#5-tui-и-пользовательский-опыт)
6. [CLI и парсинг флагов](#6-cli-и-парсинг-флагов)
7. [Тестирование](#7-тестирование)
8. [Качество кода и мелочи](#8-качество-кода-и-мелочи)


---

## 1. Безопасность

✅ **Все пункты выполнены.**

- **1.1 Path Traversal** — добавлена `validateTargetPath`, проверяющая `filepath.Rel` перед `os.MkdirAll`.
- **1.2 Symlink attack** — в `Copy` используется `os.Lstat` + удаление symlink перед копированием; `copyFile` использует атомарный `os.CreateTemp` + `os.Rename`.
- **1.3 HashFile named pipe** — перед открытием проверяется `info.Mode().IsRegular()`.


---

## 2. Производительность

✅ **Все пункты выполнены.**

- **2.1 Sync() на каждый файл** — `tmpFile.Sync()` убран из `copyFile`; добавлен `syncDir(c.targetRoot)` один раз в конце `Copy`.
- **2.2 nil слайс без capacity** — `files` инициализируется с `make([]FileInfo, 0, 1024)`.

---

## 3. Архитектура и связанность

✅ **Все пункты выполнены.**

- **3.1 HashFile в hasher** — `HashFile` вынесен в `internal/hasher`; `deduper` и `copier` зависят от `hasher`.
- **3.2 dirBrowser дедупликация** — выделен обобщённый `dirBrowserModel` в `tui/dirbrowser.go`; `sourcesModel` и `targetModel` используют встраивание.
- **3.3 Дублирование статистики** — добавлен `runner.Result.Stats() → ResultStats`; `cmd/main.go` и `tui/scan.go` используют единый метод.
- **3.4 Хрупкая эвристика unsorted** — добавлена константа `sorter.UnsortedDir` и функция `sorter.IsUnsorted`; заменены все `strings.Contains`.
- **3.5 Мёртвое поле DryRun** — убрано из `runner.Config`.
- **3.6 Двойное ToLower** — убраны лишние `strings.ToLower` из `isJPEG` и `isVideo`; задокументирован контракт lowercase Ext.


---

## 4. Обработка ошибок и надёжность

✅ **Все пункты выполнены.**

- **4.1 Бесшумная потеря лога** — в `tui/copy.go` добавлено поле `logErr string` в `copyModel`. Если `logger.New` возвращает ошибку (нет прав на запись), предупреждение `⚠ Не удалось создать лог: ...` отображается на экране копирования при завершении/ошибке.
- **4.2 Игнорирование `enc.Encode`** — в `cmd/main.go` заменено `_ = enc.Encode(report)` на проверку ошибки. При ошибке записи в `stdout` (pipe broken) программа выводит сообщение в `stderr` и выходит с кодом 1.
- **4.3 Ошибка `os.ReadDir`** — `loadDirItems` теперь возвращает `([]dirItem, error)`. В `dirBrowserModel` добавлено поле `readErr string`. При ошибке чтения директории (permission denied) вместо пустого списка отображается `Ошибка чтения: ...` на экранах выбора источника и цели.

---

## 5. TUI и пользовательский опыт

✅ **Все пункты выполнены.**

- **5.1 Восстановление терминала при panic** — добавлен `defer recover` в `tui/tui.go:Run`, который выводит `\x1b[?1049l` перед повторным `panic`.
- **5.2 Мультиселект источников** — `Model.Source string` заменён на `Sources []string`. В `sourcesModel` добавлен `selected map[string]struct{}`. Пробел добавляет/удаляет папку из списка; галочки `✓` отображаются в UI. Список выбранных показывается на всех экранах.
- **5.3 Несогласованные дефолты** — создан `internal/config/config.go` с константами `DefaultTemplate = "2006-01-02"`, `DefaultLivePhotos = true`, `DefaultIncludeVideo = true`, `DefaultUseMTime = true`. Используются в `cmd/main.go` и `tui/settings.go`.
- **5.4 Вводящий в заблуждение help** — текст help Live Photos изменён на `"Не считать .heic + .mov дубликатами (Live Photos)"`.
- **5.5 Некорректная замена `rel` на `/`** — убран `if rel == "." { rel = "/" }` в `tui/preview.go`.

---

## 6. CLI и парсинг флагов

✅ **Все пункты выполнены.**

- **6.1 Проверка директории source** — в цикле валидации `sources` добавлена проверка `info.IsDir()`. Если передан файл вместо папки — ошибка до запуска pipeline.
- **6.2 Проверка target на запись** — перед запуском выполняется `os.MkdirAll(target, 0755)` и тестовая запись через `os.CreateTemp(target, ".write-test-*")`. Права проверяются до сканирования.
- **6.3 Частичная статистика при ошибке копирования** — если `c.Copy` возвращает ошибку, программа сначала выводит отчёт (`printTextReport` или `printJSONReport`) с накопленной статистикой, а затем выходит с кодом 1.

---

## 7. Тестирование

### 7.1 Пакет `logger` — полностью без тестов

**Проблема:** нет ни одного `*_test.go`.

**Как исправить:** добавить тесты на `Log`, `Close`, race condition, обработку ошибок записи.

### 7.2 `internal/scanner` — Нет тестов на edge cases

**Проблема:** нет тестов на:
- несуществующий `source`
- permission denied
- пустую директорию
- симлинки
- отмену через `context.Context`
- большое количество файлов (perf)

**Как исправить:** добавить табличные тесты с временными директориями.

### 7.3 `internal/deduper` — Нет тестов на ошибки хеширования

**Проблема:** нет теста на `HashFile` с `permission denied`, удалённым файлом, named pipe.

**Как исправить:** использовать `os.Chmod` / `os.Remove` в тестах для симуляции ошибок.

### 7.4 `internal/dateresolver` — Нет тестов на command injection и concurrent вызовы

**Проблема:** нет теста на имя файла с `-`, нет теста на таймаут `exiftool`.

**Как исправить:** создать файл с именем `-test.txt` и проверить, что `extractVideoDate` не падает.

### 7.5 `internal/copier` — Нет тестов на critical paths

**Проблема:** нет тестов на:
- symlink attack
- partial write cleanup
- `not enough disk space` (mock `availableSpace`)
- context cancel во время `io.Copy`
- path traversal

**Как исправить:**
- Вынести `availableSpace` в интерфейс для мокирования.
- Добавить `TestCopy_ContextCancelDuringCopy` с медленным `io.Pipe`.

### 7.6 `cmd/main_test.go` — Хрупкая проверка `dry-run`

**Проблема:** `filepath.Glob` не проверяет подпапки. Ошибка `Glob` игнорируется.

**Как исправить:** использовать `os.ReadDir` рекурсивно или `filepath.WalkDir`.

### 7.7 Отсутствуют benchmark и fuzz тесты

**Проблема:** нет `BenchmarkHashFile`, `BenchmarkFindDuplicates`, `FuzzResolveDate`.

**Как исправить:** добавить базовые bench и fuzz тесты для критичных функций.

### 7.8 `internal/integration_test.go` — Завязан на точное количество файлов

**Проблема:** `TestRun_EndToEnd` проверяет ровно 12 файлов в `testdata/e2e/source/2023`. Добавление новой фикстуры сломает тест.

**Как исправить:** использовать `filepath.WalkDir` для подсчёта ожидаемого количества файлов на лету, либо хранить ожидаемые результаты в JSON-файле рядом с фикстурами.


---

## 8. Качество кода и мелочи

✅ **Все пункты выполнены.**

- **8.1 Мёртвый код uppercase** — уже очищен в `internal/dateresolver/video.go` в рамках раздела 3.6. Остались только lowercase кейсы с комментарием о контракте lowercase `Ext`.
- **8.2 Магическая строка `"unsorted"`** — вынесена в константу `sorter.UnsortedDir` и функцию `sorter.IsUnsorted` в рамках раздела 3.4.
- **8.3 Защита `findFreeName` от переполнения** — добавлен `const maxIterations = 10000`. Функция возвращает `(string, error)`; при превышении лимита возвращается ошибка, которая обрабатывается в `Copy` как обычная ошибка копирования.
- **8.4 `ErrorList` как `[]error`** — `copier.Stats.ErrorList` изменён на `[]error`. `recordError` сохраняет тип ошибки. Для JSON/текста добавлена функция `errorStrings()`. В `tui/copy.go` лог использует `e.Error()`.
- **8.5 `default` в switch по экранам** — в `tui/model.go` добавлен `default: panic(fmt.Sprintf("unknown screen: %d", m.screen))` в `Update` и `View`.
- **8.6 Magic numbers в `filename.go`** — в `parsePXL` добавлены именованные константы `pxlPrefixLen`, `pxlCoreLen`, `pxlCoreEnd`, `pxlTotalLen`.
- **8.7 Дублирование подсчёта unsorted** — в `cmd/main.go` выделена функция `collectUnsortedFiles(entries []sorter.Entry, useBase bool) []string`, используемая в `printTextReport` и `printJSONReport`.
- **8.8 `findPreset` завязан на порядок** — в `templatePreset` добавлен флаг `isCustom bool`. `findPreset` ищет пользовательский пресет по `isCustom`, а не по индексу последнего элемента.

---

## Приоритетный план действий (рекомендация по порядку исправления)

### P1 — Средний (производительность и UX)
1. `copier.go` — убрать `Sync()` на каждый файл (опциональный `--fsync`).
2. `scanner.go` — инициализировать слайс с capacity.

### P2 — Низкий (рефакторинг и мелочи)
3. Дедупликация `sources.go` / `target.go` → `dirBrowserModel`.
4. Вынести общие дефолты в `internal/config`.
5. Добавить benchmark и fuzz тесты.
6. Убрать магические строки/числа в константы.

---

*Документ составлен на основе полного аудита кодовой базы. Каждый пункт содержит конкретное место в коде, объяснение проблемы и рекомендацию по исправлению.*
