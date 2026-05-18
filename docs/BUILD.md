# Сборка и релизы

## Сборка из исходников

### Через Makefile

```bash
git clone https://github.com/dmitriy-dorofeev/photo-sorter.git
cd photo-sorter
make build
```

Результат появится в `bin/photo-sorter`. Версия автоматически подставится из текущего git-тега или хеша коммита.

### Вручную через go build

```bash
go build -ldflags "-X main.version=$(git describe --tags --always --dirty)" -o photo-sorter ./cmd
```

> Без `-ldflags` приложение сообщит версию `dev`, и автообновление будет недоступно.

## macOS .app bundle

Для удобства на macOS можно собрать приложение как `.app` bundle с иконкой в Finder.

```bash
make build-mac-app
```

Результат:
- `bin/Photo Sorter.app/` — готовый bundle
- `bin/Photo Sorter.app.zip` — zip-архив для распространения

### Иконка

Чтобы у bundle была иконка, положите PNG размером **1024×1024** в `build/macos/icon.png` перед сборкой. Скрипт автоматически сконвертирует её в `.icns`.

### Как запускать

1. Распакуйте `Photo Sorter.app.zip`.
2. (Опционально) Перетащите `Photo Sorter.app` в `Applications`.
3. Двойной клик по иконке — откроется **Terminal** и в нём запустится TUI.

> Поскольку photo-sorter — консольное TUI-приложение, `.app` bundle открывает Terminal. Это ожидаемое поведение.

### Обновление внутри .app bundle

Команда `update` работает и из `.app`, но с ограничениями:
- Обновляется только бинарник внутри `Photo Sorter.app/Contents/MacOS/photo-sorter`.
- Wrapper, `Info.plist` и иконка останутся от первоначальной версии.
- Если приложение установлено в `/Applications/`, могут потребоваться права администратора.

При мажорных обновлениях рекомендуется скачивать новый `.app.zip` вручную.

## Версионирование

Проект следует [Semantic Versioning](https://semver.org/lang/ru/): `vMAJOR.MINOR.PATCH`.

```bash
./photo-sorter --version
```

## Автоматическое обновление

Проверить наличие новой версии:

```bash
./photo-sorter --check-update
```

Установить последнюю версию из GitHub Releases:

```bash
./photo-sorter update
```

> При обновлении старая версия сохраняется с суффиксом `.bak`. Для сборок из исходников (`dev`) автообновление недоступно.

## Публикация релиза

Релизы создаются автоматически при пуше git-тега, начинающегося на `v`:

```bash
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

GitHub Actions запустит [GoReleaser](https://goreleaser.com/), который:
- соберёт бинарники для **macOS** (Intel + Apple Silicon) и **Linux** (x86_64 + ARM64);
- упакует их в архивы `.tar.gz`;
- для **macOS** дополнительно создаст `.app.zip` с иконкой и bundle;
- сгенерирует `checksums.txt`;
- создаст страницу Release на GitHub с changelog.

Также можно запустить бамп версии через GitHub UI: **Actions → Bump Version → Run workflow**.

## Локальная сборка snapshot

```bash
make snapshot
```

Требуется установленный [GoReleaser](https://goreleaser.com/install/). Артефакты появятся в папке `dist/`.

## Структура проекта

```
photo-sorter/
├── cmd/              # точка входа (TUI по умолчанию, CLI через флаги)
├── internal/         # приватные пакеты
│   ├── scanner/      # обход папок
│   ├── dateresolver/ # движок дат (EXIF, видео, имена, mtime)
│   ├── deduper/      # поиск дубликатов
│   ├── sorter/       # построение целевого дерева
│   ├── copier/       # безопасное копирование
│   ├── logger/       # логи
│   ├── notify/       # системные уведомления
│   ├── runner/       # единый pipeline
│   └── updater/      # автообновление
├── tui/              # интерфейс на bubbletea
└── testdata/         # тестовые файлы
```

## Тестирование

```bash
# Все тесты
go test ./...

# С подробным выводом
go test -v ./...

# Интеграционные тесты
go test ./internal/ -run TestEndToEnd -v
```
