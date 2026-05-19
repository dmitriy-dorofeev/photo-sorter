# Аудит проекта на соответствие Go-паттернам

Дата: 2026-05-19

Ниже зафиксированы результаты экспресс-аудита по исходникам на соответствие idiomatic Go и устойчивым паттернам (ошибки, `context`, lifecycle ресурсов, pipeline-границы).

## Findings

### 1) High: утечка/незакрытие `state` в CLI-пути

`runner.Run` возвращает открытый `res.State`, но в `runCLI` он нигде не закрывается. Это риск lock-файла BoltDB и накопления ресурсов.

- [internal/runner/runner.go](/Users/dimulka/GolandProjects/photo-sorter/internal/runner/runner.go:78)
- [cmd/main.go](/Users/dimulka/GolandProjects/photo-sorter/cmd/main.go:293)

### 2) High: неполная межзапусковая дедупликация в `deduper`

Если хеш уже известен (`knownHashes`), код помечает только `candidate`, но не `original`. В группе это может привести к лишнему копированию дубликата, который уже есть в target/state.

- [internal/deduper/deduper.go](/Users/dimulka/GolandProjects/photo-sorter/internal/deduper/deduper.go:125)

### 3) Medium: `context`-cancel в `sorter.BuildTree` даёт “тихий” partial result

При `ctx.Err()` цикл просто `break`, ошибка наружу не возвращается. Это неочевидное поведение для pipeline-этапа и усложняет диагностику.

- [internal/sorter/sorter.go](/Users/dimulka/GolandProjects/photo-sorter/internal/sorter/sorter.go:94)

### 4) Medium: слишком широкое `recover()` в `runner.Run`

Библиотечный слой гасит любые panic и отдаёт `error` без stack trace. Для Go-паттернов обычно лучше падать явно (или логировать stack и re-panic на границе процесса).

- [internal/runner/runner.go](/Users/dimulka/GolandProjects/photo-sorter/internal/runner/runner.go:80)

### 5) Medium: хрупкий парсинг даты из `exiftool` (без timezone/layout-вариантов)

Парсится только `2006:01:02 15:04:05`; значения с зоной/субсекундами будут отброшены, хотя метаданные есть.

- [internal/dateresolver/video.go](/Users/dimulka/GolandProjects/photo-sorter/internal/dateresolver/video.go:56)

## Ограничения проверки

- Тесты не были валидированы в этой среде: локальный Go SDK некорректен (`package ... is not in std`).
- `rg` в среде отсутствует, использовались `find/sed/nl`.

## Краткая оценка

Структура пакетов, декомпозиция pipeline и разделение TUI/доменной логики — хорошие и go-идиоматичные. Основные риски сейчас в lifecycle state и граничных сценариях дедупликации/отмены.
