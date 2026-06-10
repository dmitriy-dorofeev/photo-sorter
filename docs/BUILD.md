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
- сгенерирует `checksums.txt`;
- создаст страницу Release на GitHub с changelog;
- автоматически обновит формулу в [homebrew-tap](https://github.com/dmitriy-dorofeev/homebrew-tap).

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
│   ├── report/       # генерация итогового отчёта (text / html)
│   ├── notify/       # системные уведомления
│   ├── runner/       # единый pipeline
│   ├── updater/      # автообновление
│   ├── facedetect/   # детекция лиц (YuNet ONNX)
│   ├── facerecogn/   # распознавание лиц (ArcFace ONNX)
│   ├── facecluster/  # кластеризация лиц (Chinese Whispers)
│   ├── facealias/    # алиасы кластеров
│   └── facerunner/   # оркестрация face-режима
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
