# Установка

## Требования

- **Go 1.25+** — только для сборки из исходников
- **`exiftool`** — опционально, но рекомендуется для видео и записи EXIF
- **`onnxruntime`** — опционально, только для режима сортировки по лицам (`--sort-mode=face`)

## Установка через Homebrew (рекомендуется)

```bash
brew tap dmitriy-dorofeev/tap
brew install photo-sorter
```

Homebrew-формула поддерживает macOS (Intel и Apple Silicon) и Linux (x86_64, ARM64).

## Установка бинарника вручную

Готовые бинарники доступны на странице [Releases](https://github.com/dmitriy-dorofeev/photo-sorter/releases).

```bash
# Пример для macOS Apple Silicon
curl -LO https://github.com/dmitriy-dorofeev/photo-sorter/releases/latest/download/photo-sorter_Darwin_arm64.tar.gz
tar -xzf photo-sorter_Darwin_arm64.tar.gz
./photo-sorter --version
```

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

## Установка ONNX Runtime

Требуется только для режима **сортировки по лицам** (`--sort-mode=face`).

```bash
# macOS через Homebrew
brew install onnxruntime

# Ubuntu / Debian
wget https://github.com/microsoft/onnxruntime/releases/download/v1.20.1/onnxruntime-linux-x64-1.20.1.tgz
tar -xzf onnxruntime-linux-x64-1.20.1.tgz
sudo cp onnxruntime-linux-x64-1.20.1/lib/libonnxruntime.so.1.20.1 /usr/local/lib/
sudo ldconfig
```

## ONNX-модели для face-режима

При первом запуске face-режима (`--sort-mode=face`) приложение автоматически скачает необходимые ONNX-модели в `~/.photo-sorter/models/`:
- `face-detection.onnx` (YuNet, ~233 KB)
- `face-recognition.onnx` (ArcFace MobileFaceNet, ~13 MB)

Чтобы использовать другую папку для моделей, укажите `--face-model-path` (CLI).

> Если `onnxruntime` не установлен, режим `--sort-mode=face` будет недоступен, но сортировка по датам (`--sort-mode=date`, по умолчанию) продолжит работать.

> Если `exiftool` не установлен, видео всё равно будут обработаны через парсинг имени файла или `mtime`, а запись EXIF будет недоступна.

## Проверка зависимостей

```bash
./photo-sorter --check-deps
```

Выведет таблицу со статусом каждой зависимости и подсказки по установке для вашей ОС.
