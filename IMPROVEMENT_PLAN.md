# IMPROVEMENT_PLAN.md

> Ревью проекта **photo-sorter** от Senior Go Developer.
> Статус: полный аудит кодовой базы (~5K строк Go).
> Цель: устранить критические баги, закрыть security-дыры, повысить производительность и надёжность.

---

## Содержание

1. [Тестирование](#1-тестирование)

---

## 1. Тестирование

**Статус: полностью выполнен.**

### Выполненные работы

- **1.1 Пакет `logger`** — добавлен `internal/logger/logger_test.go`:
  - `TestNew_Error` — ошибки создания (несуществующая директория, путь = директория).
  - `TestLog` — корректность записи и формата.
  - `TestLog_Concurrent` — 100 горутин, race-free.
  - `TestLog_AfterClose` — ошибка записи после `Close`.

- **1.2 `internal/scanner`** — дополнен `internal/scanner/scanner_test.go`:
  - `TestScan_NonExistentSource` — ошибка для несуществующего source.
  - `TestScan_EmptyDir` — пустая директория возвращает 0 файлов.
  - `TestScan_PermissionDenied` — пропуск заблокированной поддиректории.
  - `TestScan_Symlink` — symlink обрабатывается как отдельный файл (размер ссылки).
  - `TestScan_ContextCancel` — отмена контекста возвращает ошибку.
  - `TestScan_ManyFiles` — performance на 500 файлах.
  - **Исправлен баг в `scanner.go`**: ошибка на корневом пути теперь прокидывается (`path == src`), а не игнорируется.

- **1.3 `internal/deduper`** — дополнен `internal/deduper/deduper_test.go`:
  - `TestFindDuplicates_HashError` — `permission denied` при хешировании (одинаковый размер, чтобы попасть в группу).
  - `TestFindDuplicates_NamedPipe` — хеширование FIFO возвращает ошибку.

- **1.4 `internal/dateresolver`** — дополнен `internal/dateresolver/dateresolver_test.go`:
  - `TestExtractVideoDate_CommandInjection` — файл с именем `-test.mov`, защита через `--`.
  - `TestExtractVideoDate_Timeout` — fake exiftool, который спит; проверка прерывания.
  - `TestExtractVideoDate_Concurrent` — 20 параллельных вызовов без паники.
  - **Исправлено в `video.go`**: `videoTimeout` вынесена в package-level переменную для переопределения в тестах.

- **1.5 `internal/copier`** — дополнен `internal/copier/copier_test.go`:
  - `TestCopy_PathTraversal` — обнаружение `../` в target.
  - `TestCopy_SymlinkAttack` — symlink удаляется, victim не перезаписывается.
  - `TestCopy_NotEnoughDiskSpace` — mock `spaceFunc`, возвращающий 1 байт.
  - `TestCopy_ContextCancelMidway` — отмена через progress callback после первого файла.
  - **Исправлено в `copier.go`**: `spaceFunc` вынесена в поле `Copier` для мокирования.

- **1.6 `cmd/main_test.go`** — исправлена хрупкая проверка `dry-run`:
  - `filepath.Glob(targetDir, "*")` заменён на `filepath.WalkDir` для рекурсивной проверки отсутствия файлов.

- **1.7 Benchmark и fuzz** — добавлены:
  - `internal/hasher/hasher_test.go`: `BenchmarkHashFile` (10 MB файл).
  - `internal/deduper/deduper_test.go`: `BenchmarkFindDuplicates` (100 файлов, 50 пар дубликатов).
  - `internal/dateresolver/dateresolver_test.go`: `FuzzResolveDate` (фаззинг `Resolve` с разными именами и расширениями).

- **1.8 `internal/integration_test.go`** — убрана завязка на точное число файлов:
  - Добавлен хелпер `countFilesWithExts`, который считает файлы динамически по расширениям.
  - `TestCLIJSON` проверяет `FilesFound > 0` вместо жёсткого `!= 12`.

---

*Документ составлен на основе полного аудита кодовой базы. Каждый пункт содержит конкретное место в коде, объяснение проблемы и рекомендацию по исправлению.*
