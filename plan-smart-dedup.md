# План реализации: Умная обработка дубликатов

> Фича из `roadmap.md`, раздел 3.  
> Цель: дать пользователю выбор стратегии выбора «оригинала» из группы дубликатов, улучшить логирование и заложить основу для слияния метаданных.

---

## 1. Текущая реализация (baseline)

- **Группировка**: `deduper.FindDuplicates` → size → xxhash → группы.
- **Выбор оригинала**: жёстко зашито — детерминированная сортировка по пути (`Path <`).
- **Пропуск дублей**: все остальные файлы группы помечаются `Skip = true` в `sorter.BuildTree`.
- **Логирование**: дубли отображаются в JSON/text-отчёте CLI, но в файл-лог не попадают.

---

## 2. Стратегии дедупликации (MVP)

| Стратегия | Правило выбора оригинала | Обоснование |
|-----------|--------------------------|-------------|
| `path` | По алфавиту пути (текущее поведение) | Обратная совместимость |
| `largest` | Максимальный `Size` | Больший файл → меньше сжатие / лучше качество |
| `newest` | Максимальный `ModTime` | Новее → предположительно меньше обработок |
| `best-meta` | Приоритет источника даты: EXIF → видео-мета → имя → mtime → none | Сохраняем файл с самой «настоящей» датой съёмки |

### Про «слияние метаданных»

Проект следует принципу **только копирование**, без модификации исходников.  
Поэтому в рамках этой фичи «слияние» интерпретируется как:

> Выбрать в качестве оригинала тот файл, у которого метаданные богаче, а в лог записать разницу между оригиналом и пропущенными дубликатами.

Физическая перезапись EXIF (запись `DateTimeOriginal` через exiftool) — это отдельная фича roadmap («Обратная синхронизация метаданных»).

---

## 3. Архитектурные изменения

### 3.1. `internal/deduper/strategy.go` (новый файл)

```go
type Strategy string

const (
    StrategyPath     Strategy = "path"
    StrategyLargest  Strategy = "largest"
    StrategyNewest   Strategy = "newest"
    StrategyBestMeta Strategy = "best-meta"
)

// Source — приоритет источника даты (заполняется dateresolver).
type Source int

const (
    SourceNone Source = iota
    SourceModTime
    SourceFilename
    SourceVideo
    SourceExif
)

// PickOriginal выбирает лучший файл из группы дубликатов.
func PickOriginal(
    files []scanner.FileInfo,
    strategy Strategy,
    dateSources map[string]Source,
) scanner.FileInfo
```

**Правила fallback** (гарантируют детерминированность):
1. При равенстве размеров (`largest`) → fallback на `path`.
2. При равенстве времени (`newest`) → fallback на `largest`, затем на `path`.
3. При равенстве `Source` (`best-meta`) → fallback на `largest`, затем на `path`.
4. Если `dateSources` не передана → fallback на `largest`.

### 3.2. `internal/deduper/deduper.go`

- Добавить поля в `Deduper`:
  ```go
  strategy    Strategy
  dateSources map[string]Source
  ```
- Обновить конструктор:
  ```go
  func New(
      files []scanner.FileInfo,
      livePhotos bool,
      strategy Strategy,
      dateSources map[string]Source,
  ) *Deduper
  ```
- В `FindDuplicates` вместо `sort.Slice(hashGroup, ...)` по `Path` использовать `PickOriginal(hashGroup, d.strategy, d.dateSources)`.
- Live Photos: исключение пар `.HEIC`/`.MOV` происходит **до** применения стратегии (текущая логика `isLivePhotoPair` не меняется).

### 3.3. `internal/dateresolver/dateresolver.go`

- Экспортировать `type Source int` с константами (сейчас `Resolve` возвращает только `(time.Time, bool)`).
- Добавить метод `ResolveWithSource(ctx, file) (time.Time, bool, Source)` или кешировать источник внутри `DateResolver`.
- `runner` будет собирать `map[string]Source` после `ResolveBatch`.

### 3.4. `internal/runner/runner.go`

```go
type Config struct {
    Sources      []string
    Target       string
    Template     string
    LivePhotos   bool
    IncludeVideo bool
    UseMTime     bool
    DupStrategy  string // "path" | "largest" | "newest" | "best-meta"
}
```

- После `dr.ResolveBatch` построить `map[string]dateresolver.Source`.
- Передать стратегию и мапу в `deduper.New`.

### 3.5. CLI: `cmd/main.go`

- Новый флаг: `--dup-strategy string` (`path`, `largest`, `newest`, `best-meta`).
- Дефолт: `path`.
- Валидация: если значение не из списка — ошибка.

### 3.6. TUI: `tui/settings.go`

Текущий `settingType` поддерживает только `bool` и `text`. Нужен новый тип:

```go
type settingType int

const (
    settingTypeText settingType = iota
    settingTypeBool
    settingTypeChoice // новый: циклический выбор из списка
)

type setting struct {
    label     string
    key       string
    help      string
    stype     settingType
    stringValue string
    boolValue   bool
    choices     []string // для settingTypeChoice
    choiceIdx   int
}
```

- Добавить пункт «Стратегия дедупликации»:
  - key: `dup_strategy`
  - choices: `["По имени файла", "По размеру", "По дате изменения", "По метаданным"]`
  - маппинг choice → `runner.Config.DupStrategy` в `tui/scan.go`.

### 3.7. Логирование дубликатов

Сейчас `logger.Logger` — простой `Log(string)`. Предлагается добавить:

```go
func (l *Logger) LogDuplicate(original, duplicate string, strategy Strategy) error
```

Вызывать из `runner.Run` (или `copier.Copy`) после построения дерева. Пример записи:

```
[2026-05-12T19:10:00Z] DUPLICATE: kept /src/photo_1.jpg (strategy=largest), skipped /src/photo_2.jpg (same hash)
```

Это закрывает пункт roadmap «…или записать в лог».

---

## 4. Порядок реализации

| Этап | Что делать | Сложность | Зависимости |
|------|------------|-----------|-------------|
| **1** | `Strategy` + `PickOriginal` + unit-тесты (`path`, `largest`, `newest`) | Low | Нет |
| **2** | Интеграция в `deduper`, `runner`, CLI-флаг `--dup-strategy` | Low | Этап 1 |
| **3** | TUI: `settingTypeChoice` + экран настроек | Medium | Этап 2 |
| **4** | `dateresolver.Source` + стратегия `best-meta` | Medium | Этап 2 |
| **5** | Расширенное логирование дубликатов в `logger`/`copier` | Low | Этап 2 |

---

## 5. Граничные случаи

- **Равенство критериев**: всегда есть fallback на `path` — исключаем недетерминированность.
- **Отсутствие `dateSources`**: если мапа `nil` (например, старая версия вызова), fallback на `largest`.
- **Live Photos**: пары `.HEIC`+`.MOV` по-прежнему исключаются из дублей до применения стратегии.
- **Обратная совместимость**: дефолт `path` гарантирует идентичное поведение существующих тестов и UI.
- **Ошибка хеширования**: файл, который не удалось захешировать, исключается из группы (текущее поведение).

---

## 6. Файлы, подлежащие изменению

```
internal/deduper/strategy.go          # новый
internal/deduper/deduper.go           # +strategy +dateSources
internal/deduper/deduper_test.go      # параметризованные тесты
internal/dateresolver/dateresolver.go # +Source
internal/runner/runner.go             # +DupStrategy, проброс в deduper
internal/runner/runner_test.go        # тесты Config
internal/config/config.go             # +DefaultDupStrategy
internal/logger/logger.go             # +LogDuplicate
cmd/main.go                           # +--dup-strategy
tui/settings.go                       # +settingTypeChoice
tui/scan.go                           # проброс DupStrategy из UI
```

---

*Документ составлен для фичи «Умная обработка дубликатов» из roadmap.*
