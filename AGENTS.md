# photo-sorter — руководство для AI-агентов

## Обзор проекта

**photo-sorter** — консольное TUI-приложение на Go для организации фотографий и видео с разных устройств (iPhone, Android, компьютер и т.д.). Приложение сканирует исходные папки, определяет дату съёмки, пропускает дубликаты, генерирует новые имена файлов по шаблону и копирует файлы в целевую папку по структуре дат (например, `YYYY/MM-DD/`).

Все комментарии в коде, UI-тексты и документация проекта написаны на русском языке.

## Технологический стек

- **Язык**: Go 1.25.0 (минимальная требуемая версия — 1.25+)
- **TUI-фреймворк**: [Charm Bracelet](https://charm.sh/) — `bubbletea` + `bubbles` + `lipgloss`
- **EXIF**: `github.com/rwcarlsen/goexif` (только JPEG)
- **Хеширование**: `github.com/cespare/xxhash/v2` (дедупликация)
- **Параллелизм**: `golang.org/x/sync/errgroup`
- **Системные вызовы**: `golang.org/x/sys/unix` (проверка свободного места на диске)
- **Видео-метаданные**: чтение через внешний `exiftool` (реализовано)

## Структура проекта

```
photo-sorter/
├── cmd/
│   ├── main.go                    # Точка входа: TUI или CLI
│   ├── main_test.go               # CLI-тесты (help, json, validation)
│   └── update.go                  # Логика self-update (подкоманда update)
├── internal/                      # Приватные пакеты (стандартный Go layout)
│   ├── runner/
│   │   ├── runner.go              # Единый pipeline scan → dedup → sort для TUI и CLI
│   │   └── runner_test.go
│   ├── scanner/
│   │   ├── scanner.go             # Рекурсивный обход папок, фильтрация по расширениям
│   │   └── scanner_test.go
│   ├── dateresolver/
│   │   ├── dateresolver.go        # Приоритет: EXIF → видео-метаданные → имя файла → mtime → unsorted
│   │   ├── exif.go                # Чтение EXIF из JPEG через goexif
│   │   ├── video.go               # Чтение метаданных видео через exiftool
│   │   ├── filename.go            # Реестр из 8 парсеров имён файлов
│   │   ├── dateresolver_test.go
│   │   └── video_test.go
│   ├── deduper/
│   │   ├── deduper.go             # Двухуровневая дедупликация: размер → xxhash
│   │   ├── hasher.go              # Потоковое вычисление xxhash
│   │   └── deduper_test.go
│   ├── renamer/
│   │   ├── renamer.go             # Шаблонизатор имён файлов (плейсхолдеры: {YYYY}, {original}, {device}, {seq} и др.)
│   │   ├── device.go              # Эвристическое определение устройства по имени файла
│   │   ├── renamer_test.go
│   │   └── device_test.go
│   ├── sorter/
│   │   ├── sorter.go              # Построение целевого дерева, генерация имён, разрешение коллизий
│   │   └── sorter_test.go
│   ├── copier/
│   │   ├── copier.go              # Копирование с проверкой диска, отмена, post-copy проверка целостности (xxhash)
│   │   └── copier_test.go
│   ├── logger/
│   │   └── logger.go              # Запись логов операций с timestamp
│   ├── updater/
│   │   ├── updater.go             # Проверка и установка обновлений с GitHub Releases
│   │   └── updater_test.go
│   └── integration_test.go        # E2E-тест: полный pipeline scanner → copier
├── tui/                           # Интерактивный интерфейс на bubbletea
│   ├── tui.go                     # Запуск программы bubbletea
│   ├── model.go                   # Главная модель с 6 экранами
│   ├── sources.go                 # Экран 1: выбор исходной папки (браузер)
│   ├── target.go                  # Экран 2: выбор целевой папки (браузер)
│   ├── settings.go                # Экран 3: настройки (шаблон, флаги)
│   ├── scan.go                    # Экран 4: сканирование с прогресс-баром
│   ├── preview.go                 # Экран 5: предпросмотр дерева, дублей, unsorted
│   ├── copy.go                    # Экран 6: копирование с прогрессом и логированием
│   └── styles.go                  # Цветовые стили lipgloss
├── testdata/                      # Фикстуры для тестов
│   ├── dateresolver/minimal.jpg   # JPEG с реальными EXIF-данными (2024-03-15)
│   ├── deduper/                   # Бинарные файлы для тестов хеширования и дедупликации
│   └── e2e/source/                # Полный набор файлов для integration_test.go
├── Makefile                       # Команды build, test, snapshot
├── .goreleaser.yaml               # Конфигурация GoReleaser
├── .github/workflows/release.yml  # CI: автоматический релиз по git-тегу
├── bin/                           # Директория для собранного бинарника
├── go.mod
├── go.sum
└── README.md                      # Пользовательская документация
```

## Сборка и запуск

### Автоматические релизы

Вместо ручного создания тега можно использовать:

```bash
# Локально — через Makefile
make release-patch   # v0.1.0 → v0.1.1
make release-minor   # v0.1.0 → v0.2.0
make release-major   # v0.1.0 → v1.0.0
```

После пуша тега автоматически запускается `Release`-workflow (GoReleaser).

Через GitHub UI: **Actions → Bump Version → Run workflow** — выбрать `patch` / `minor` / `major`. Workflow создаст тег и сразу запустит GoReleaser (не ждёт отдельного триггера `push: tags`).

### Требования

- Go 1.25+
- `exiftool` (опционально, рекомендуется для видео)

### Команды

```bash
# Сборка бинарника (через Makefile, с встраиванием версии)
make build

# Сборка .app bundle для macOS (с иконкой в Finder)
# Положите иконку 1024×1024 в build/macos/icon.png перед сборкой
make build-mac-app

# Или вручную с указанием версии
go build -ldflags "-X main.version=$(git describe --tags --always --dirty)" -o photo-sorter ./cmd

# Проверка версии
./photo-sorter --version

# Проверить наличие обновлений
./photo-sorter --check-update

# Автоматическое обновление до последней версии
./photo-sorter update

# Запуск в TUI-режиме (по умолчанию)
./photo-sorter

# CLI-режим
./photo-sorter --source ./photos --target ./sorted --dry-run
./photo-sorter --source ./a --source ./b --target ./out --dry-run=false
./photo-sorter --source ./photos --target ./sorted --format=json
./photo-sorter --source ./photos --target ./sorted --name-template "{YYYY}-{MM}-{DD}_{original}{ext}"

# Запуск тестов
go test ./...

# Запуск конкретного пакета
go test ./internal/dateresolver/

# Запуск E2E-теста
go test ./internal/ -run TestEndToEnd -v
go test ./internal/ -run TestEndToEnd_UseModTime -v
go test ./internal/ -run TestCancellation -v
go test ./cmd/ -run TestCLI -v

# Локальная сборка релиза (snapshot) без публикации
# Для macOS автоматически создаётся .app bundle в dist/*.app.zip
make snapshot
```

### Навигация в TUI

- `↑/↓` — перемещение курсора
- `Enter` — открыть папку / подтвердить
- `Backspace` — вверх по папкам
- `Пробел` — выбрать источник / цель
- `←/→` — назад / вперёд по экранам
- `Esc` — выход

## Архитектура и поток данных

1. **scanner** — параллельно обходит исходные папки (`filepath.WalkDir` + `errgroup`), фильтрует по расширениям, собирает `[]FileInfo`.
2. **deduper** — группирует файлы по размеру, внутри групп вычисляет `xxhash`, исключает пары Live Photos (`.HEIC` + `.MOV` с одинаковым basename).
3. **dateresolver** — для каждого файла: EXIF (JPEG) → видео-метаданные (exiftool) → парсинг имени (8 паттернов) → `mtime` (если включено `UseModTime`) → `unsorted/`.
4. **sorter** — строит план копирования: целевой путь по шаблону даты, разрешение коллизий (`_1`, `_2`), пометка дублей как `Skip`, Live Photos fallback (`.MOV` получает дату от `.HEIC` с тем же basename).
5. **copier** — выполняет копирование: проверка свободного места (`unix.Statfs`), создание директорий, обработка внешних коллизий по хешу, **post-copy проверка целостности** (сверка xxhash исходника и копии после atomic rename), поддержка `context.Context` (отмена), progress callback.
6. **logger** — после копирования создаёт лог-файл `YYYY-MM-DD_HH-MM-SS_photo-sorter.log` в целевой папке.
7. **updater** — проверяет наличие новой версии на GitHub Releases и выполняет self-update бинарника.

## Тестирование

### Стратегия

- **Unit-тесты** — в каждом пакете `internal/*` и `internal/renamer/`: покрывают основную логику и edge cases.
- **Integration-тест** — `internal/integration_test.go`: полный сквозной тест от сканирования до копирования на фикстурах из `testdata/e2e/`.
- **Фикстуры** — реальные и сгенерированные файлы в `testdata/`:
  - `minimal.jpg` — JPEG с валидным EXIF-блоком (дата 2024-03-15 14:30:22).
  - `deduper/*.bin` — бинарные файлы разного размера и содержимого для тестов хешей.
  - `e2e/source/` — набор из 20 файлов в подпапках `2023/` и `2024/`, имитирующих разные сценарии (iPhone, Android, WhatsApp, Signal, Pixel, Screenshot, DSC, дубликаты, Live Photos, файлы без даты, видео).

### Запуск тестов

```bash
# Все тесты
go test ./...

# С подробным выводом
go test ./... -v

# Только integration
go test ./internal/ -run TestEndToEnd -v
go test ./internal/ -run TestEndToEnd_UseModTime -v
go test ./internal/ -run TestCancellation -v
```

### Особенности тестовых данных

- Файлы `dup1.jpg` и `dup2.jpg` в `e2e/source/2023/` — копии `minimal.jpg` (одинаковое содержимое, разные имена).
- `live_photo.HEIC` + `live_photo.MOV` — тестовая пара Live Photos.
- `photo_no_date.jpg` — псевдо-JPEG без EXIF и без узнаваемого имени; при `UseModTime=false` идёт в `unsorted/`.
- `video.mp4` (в `2023/`) — видео без поддерживаемых метаданных; тестирует fallback на `mtime`.
- `root_photo.jpg` (в корне `e2e/source/`) — дополнительный файл для проверки обработки корневого уровня.

## Соглашения по коду

### Стиль

- Стандартный Go formatting (`gofmt`).
- Комментарии к пакетам и публичным типам/функциям — на русском языке.
- Названия пакетов и типов — в стиле Go (PascalCase для экспортируемых, camelCase для приватных).
- Функции-конструкторы именуются `New()` или `new*Model()`.
- Приоритет дат и поведение алгоритмов документируются в комментариях к функциям.

### Организация кода

- Каждый пакет в `internal/` решает одну задачу и имеет свой тест-файл.
- TUI-экраны разделены по файлам (`sources.go`, `scan.go`, `copy.go` и т.д.), но все в пакете `tui`.
- Общие структуры данных (например, `scanner.FileInfo`) используются как контракты между пакетами.
- Прогресс и отмена длительных операций реализованы через `context.Context` и callback-функции.

### Edge cases, которые учтены в коде

- **Дубликаты** — двухуровневая проверка (размер → хеш), чтобы не хешировать все файлы.
- **Live Photos** — `.HEIC` + `.MOV` с одинаковым basename не считаются дублями друг друга; `.MOV` без даты получает дату от соответствующего `.HEIC`.
- **Коллизии имён** — если в целевой папке файл с таким именем уже есть, сравниваются хеши: совпадают → пропускаем, разные → суффикс `_1`, `_2`.
- **Файлы без даты** — помещаются в `unsorted/` в корне целевой папки. Никогда не теряются.
- **Недостаточно места** — `copier` проверяет свободное место через `unix.Statfs` перед началом копирования.
- **Отмена операции** — `copier.Copy` принимает `context.Context`, отмена работает на уровне TUI (клавиша `Esc` на экране копирования).
- **Проверка целостности после копирования** — после atomic rename `copier` сверяет `xxhash` исходника и копии. При несовпадении битый файл удаляется, операция засчитывается как ошибка (`IntegrityFailures`).

## Безопасность

- **Только копирование** — приложение никогда не перемещает и не удаляет исходные файлы.

- **Логирование** — каждый запуск сохраняет статистику в файл лога в целевой директории.
- **Unknown date → `unsorted/`** — файлы без распознаваемой даты не отбрасываются, а кладутся в отдельную папку.

## Что ещё не реализовано (TODO)

- Нет известных незавершённых задач. Все запланированные функции реализованы.

## Полезные ссылки внутри проекта

- `README.md` — пользовательская документация (установка, запуск, навигация).
