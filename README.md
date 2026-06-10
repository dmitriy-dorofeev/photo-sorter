# photo-sorter

[![Release](https://img.shields.io/github/v/release/dmitriy-dorofeev/photo-sorter)](https://github.com/dmitriy-dorofeev/photo-sorter/releases/latest)
[![CI](https://github.com/dmitriy-dorofeev/photo-sorter/actions/workflows/ci.yml/badge.svg)](https://github.com/dmitriy-dorofeev/photo-sorter/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/github/license/dmitriy-dorofeev/photo-sorter)](LICENSE)

Консольное TUI-приложение на Go для организации фотографий и видео с разных устройств (iPhone, Android, компьютер и т.д.).

## Возможности

- **Инкрементальные запуски** — повторный запуск обрабатывает только новые и изменённые файлы
- **Дедупликация** — двухуровневая проверка (размер → xxhash), включая межзапусковую
- **Определение даты** — EXIF → видео-метаданные → парсинг имени → `mtime` → `unsorted/`
- **Структура папок** — `YYYY/MM/DD/` или свой шаблон
- **Переименование по шаблону** — дата, оригинальное имя, устройство, порядковый номер
- **Live Photos** — `.HEIC` + `.MOV` группируются рядом
- **Face-кластеризация** — группировка по людям через локальные ONNX-модели (YuNet + ArcFace)
- **Обратная синхронизация EXIF** — запись `DateTimeOriginal` через `exiftool`
- **Безопасность** — только копирование, предпросмотр перед запуском, `--dry-run` по умолчанию

## Быстрый старт

### Установка

Скачайте бинарник под свою платформу со страницы [Releases](https://github.com/dmitriy-dorofeev/photo-sorter/releases/latest):

```bash
# macOS Apple Silicon
curl -LO https://github.com/dmitriy-dorofeev/photo-sorter/releases/latest/download/photo-sorter_Darwin_arm64.tar.gz
tar -xzf photo-sorter_Darwin_arm64.tar.gz
./photo-sorter --version
```

### Запуск

**Интерактивный режим (TUI):**

```bash
./photo-sorter
```

**Консольный режим (CLI):**

```bash
# Пробный прогон
./photo-sorter --source ~/Photos --target ~/Sorted --dry-run

# Реальное копирование
./photo-sorter --source ~/Photos --target ~/Sorted --dry-run=false

# Face-кластеризация (группировка по людям)
./photo-sorter --source ~/Photos --target ~/Sorted --sort-mode=face --dry-run=false
# Требуется ONNX Runtime. Модели скачиваются автоматически при первом запуске face-режима.
# Фото с несколькими людьми будет скопировано в папку каждого найденного лица.
# В TUI на экране предпросмотра можно переименовывать людей (r), смотреть примеры фото (v)
# и настроить порог сходства лиц (строгий/средний/мягкий).
```

## Документация

- [Установка и требования](./docs/INSTALL.md)
- [Использование (TUI, CLI, шаблоны)](./docs/USAGE.md)
- [Сборка из исходников и релизы](./docs/BUILD.md)

## Поддерживаемые платформы

- **macOS** — Intel и Apple Silicon
- **Linux** — x86_64, ARM64
- **BSD** — совместимость через `unix.Statfs`
- **Windows** — требуется адаптация проверки свободного места на диске

## Безопасность

- Приложение **только копирует**, никогда не перемещает и не удаляет исходные файлы.
- По умолчанию включён `--dry-run`: сначала посмотрите отчёт, затем запускайте реальное копирование.
- Файлы без распознаваемой даты попадают в папку `unsorted/`.

## Лицензия

MIT
