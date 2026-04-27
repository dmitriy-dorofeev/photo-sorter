# IMPROVEMENT_PLAN.md

> Ревью проекта **photo-sorter** от Senior Go Developer.
> Статус: полный аудит кодовой базы (~5K строк Go).
> Цель: устранить критические баги, закрыть security-дыры, повысить производительность и надёжность.

---

## 1. Конкурентность и стабильность

**Статус: ✅ Выполнено.**

## 2. Безопасность

**Статус: ✅ Выполнено.**

## 3. Надёжность и отказоустойчивость

**Статус: ✅ Выполнено.**

## 4. TUI — состояние, UX, производительность

**Статус: ✅ Выполнено.**

## 5. CLI и updater

**Статус: 🔄 Частично выполнено.**

### 5.1 ✅ CLI не ловит `Ctrl+C` / сигналы
**Файл:** `cmd/main.go`  
**Проблема:** `runner.Run` и `copier.Copy` принимают `context.Context`, но `main()` передаёт `context.Background()`. При `Ctrl+C` процесс убивается ОС, pipeline не завершается gracefully.

**Исправение:** `signal.NotifyContext` для `SIGINT`/`SIGTERM`.

### 5.2 ✅ `TestCLIHelp` сломан — Go flag возвращает exit 2
**Файл:** `cmd/main_test.go:72-97`  
**Проблема:** `flag.Parse()` при `--help` вызывает `flag.Usage()` и `os.Exit(2)`. Тест ожидает exit 0. На практике тест падает.

**Исправение:** ожидать `ExitCode() == 2` или переопределить `flag.Usage` без `os.Exit`.

### 5.3 ✅ `TestCLIVersion` хрупкий для release-версий
**Файл:** `cmd/main_test.go:19`  
**Проблема:** `!strings.Contains(output, "dev") && !strings.Contains(output, "v")` — версия `1.0.0` не содержит ни `"dev"`, ни `"v"`, тест упадёт.

**Исправение:** проверять, что вывод не пуст и не является ошибкой, или матчить через regex.

### 5.4 🟡 CLI-тесты крайне медленные — `go run` на каждый test case
**Файл:** `cmd/main_test.go`  
**Проблема:** каждый тест вызывает `exec.Command("go", "run", ".", ...)`, что запускает полную компиляцию. 10 тестов = 30–60 секунд.

**Исправение:** собирать бинарник один раз (через `TestMain` или `sync.Once`) и переиспользовать.

### 5.5 🟡 Updater: дублирование `fetchLatestRelease`
**Файл:** `cmd/update.go:141-165`, `internal/updater/updater.go:65-89`  
**Проблема:** две почти идентичные функции. Риск рассинхронизации при изменении API.

**Исправение:** оставить одну реализацию в `internal/updater`, в `cmd` просто вызывать её.

### 5.6 🟡 Updater: нет проверки целостности бинарника
**Файл:** `cmd/update.go`  
**Проблема:** скачанный tarball распаковывается и заменяет running executable без проверки checksum, signature, способности бинарника запускаться.

**Исправение:** скачивать `checksums.txt` из релиза и сверять SHA256 перед заменой.

---

## 6. Тесты и тестовые данные

### 6.1 🟡 Misleading fixture: `photo_no_date.jpg` содержит EXIF
**Файл:** `testdata/e2e/source/2023/photo_no_date.jpg`  
**Проблема:** файл byte-identical с `minimal.jpg` (506 B, реальный EXIF 2024-03-15). Имя и документация говорят "без даты", а на деле дата есть.

**Исправение:** заменить на настоящий JPEG-болванку без EXIF-сегмента.

### 6.2 🟡 Orphaned test file
**Файл:** `testdata/e2e/source/root_photo.jpg`  
**Проблема:** никакой тест не сканирует `e2e/source/` напрямую — все указывают на `source/2023/` или `source/2024/`.

**Исправение:** удалить или переместить в `2023/` (там уже есть `root_photo.jpg`).

### 6.3 🟡 `internal/hasher` — только benchmark, нет unit-тестов
**Файл:** `internal/hasher/hasher_test.go`  
**Проблема:** нет тестов на успешное хеширование, ошибку открытия, empty file, symlink rejection.

**Исправение:** добавить `TestHashFile`, `TestHashFile_NotRegular`, `TestHashFile_Empty`.

### 6.4 🟡 `tui` — полностью без тестов
**Файл:** `tui/*.go`  
**Проблема:** ~1700 строк TUI-логики не покрыты ни одним тестом.

**Исправение:** добавить минимальный набор:
- `TestNewModel` — начальное состояние.
- `TestScreenTransitions` — переходы Sources → Target → Settings и обратно.
- `TestSettingsValidation` — ввод невалидного шаблона.

### 6.5 🟡 `TestCLIJSON` скрывает stderr
**Файл:** `cmd/main_test.go:145`  
**Проблема:** использует `cmd.Output()` вместо `cmd.CombinedOutput()`. Предупреждения и ошибки от runner теряются.

**Исправение:** перейти на `CombinedOutput()`.

---

## 7. Качество кода и архитектура

### 7.1 🟡 `main()` — god function 170+ строк
**Файл:** `cmd/main.go:54-225`  
**Проблема:** flag parsing, validation, orchestration, reporting — всё в одной функции.

**Исправение:** выделить `runTUI()`, `runCLI()`, `validateInputs()`.

### 7.2 🟡 `sort` переменная затеняет пакет `sort`
**Файл:** `internal/runner/runner.go:91`  
**Проблема:** `sort := sorter.New(...)`.

**Исправение:** переименовать в `srt` или `sorterInstance`.

### 7.3 🟡 `scanStageDates` — мёртвый enum
**Файл:** `tui/scan.go:65`  
**Проблема:** значение есть, но `runner.Run` не отправляет stage `"dates"`, и в `updateScan` нет case для него.

**Исправение:** либо добавить прогресс из `dateresolver`, либо удалить.

### 7.4 🟡 `selectedItem()` — мёртвый код
**Файл:** `tui/dirbrowser.go:124-129`  
**Проблема:** нигде не вызывается.

**Исправение:** удалить.

### 7.5 🟡 Progress callback interleaving в JSON mode
**Файл:** `cmd/main.go:198-200`  
**Проблема:** `runner.Run` пишет прогресс в `os.Stderr`. Инструменты, мёржущие stdout+stderr, получат перемешанный JSON и текст.

**Исправение:** подавлять progress callback при `format == "json"`.

### 7.6 🟡 `findPreset` делает два цикла
**Файл:** `tui/settings.go:33-46`  
**Проблема:** O(2n) вместо O(n).

**Исправение:** один цикл с отслеживанием индекса custom-пресета.