# Установка

## Требования

- **Go 1.25+** — только для сборки из исходников
- **`exiftool`** — опционально, но рекомендуется для видео и записи EXIF

## Установка бинарника

Готовые бинарники доступны на странице [Releases](https://github.com/dmitriy-dorofeev/photo-sorter/releases).

```bash
# Пример для macOS Apple Silicon
curl -LO https://github.com/dmitriy-dorofeev/photo-sorter/releases/latest/download/photo-sorter_Darwin_arm64.tar.gz
tar -xzf photo-sorter_Darwin_arm64.tar.gz
./photo-sorter --version
```

Для macOS также доступен `.app` bundle — см. [BUILD.md](./BUILD.md#macos-app-bundle).

## Установка Go

Требуется только если вы собираете проект из исходников.

```bash
# macOS через Homebrew
brew install go

go version
```

## Установка exiftool

```bash
# macOS через Homebrew
brew install exiftool

exiftool -ver
```

> Если `exiftool` не установлен, видео всё равно будут обработаны через парсинг имени файла или `mtime`, а запись EXIF будет недоступна.
