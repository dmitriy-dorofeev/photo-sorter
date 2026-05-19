# Использование

Приложение работает в двух режимах: интерактивном **TUI** и консольном **CLI**.

## TUI-режим

Запустите бинарник без аргументов:

```bash
./photo-sorter
```

Интерфейс состоит из 6 экранов:

1. **Выбор источника** — браузер папок. Выберите папку с фотографиями (`Пробел`), затем `→`.
2. **Выбор цели** — выберите папку, куда скопировать отсортированные файлы.
3. **Настройки** — шаблон папок, шаблон имён файлов, режим сортировки (по датам или по людям), опции Live Photos, видео, mtime, уведомления, формат отчёта (text или HTML).
4. **Сканирование** — прогресс обработки файлов.
5. **Предпросмотр** — дерево целевой структуры, список дублей и файлов без даты.
6. **Копирование** — прогресс копирования файлов.

**Навигация:** `↑/↓` — курсор, `Enter` — открыть/подтвердить, `Backspace` — вверх по папкам, `←/→` — назад/вперёд по экранам, `Esc` — выход.

## CLI-режим

```bash
# Пробный прогон
./photo-sorter --source ~/Photos/iPhone --target /Volumes/ExternalDisk --dry-run

# Несколько источников
./photo-sorter --source ~/Photos/iPhone --source ~/Photos/Android --target /Volumes/ExternalDisk

# Реальное копирование
./photo-sorter --source ~/Photos --target ~/Sorted --dry-run=false

# Записать дату в EXIF (если дата взята из имени/mtime)
./photo-sorter --source ~/Photos --target ~/Sorted --write-exif --dry-run=false

# HTML-отчёт вместо текстового
./photo-sorter --source ~/Photos --target ~/Sorted --report-format=html --dry-run=false

# Свой шаблон имён файлов
./photo-sorter --source ~/Photos --target ~/Sorted --name-template "{YYYY}-{MM}-{DD}_{original}{ext}"

# Посмотреть все флаги
./photo-sorter --help
```

### Основные флаги

| Флаг | Описание | По умолчанию |
|------|----------|--------------|
| `--source` | Исходная папка (можно несколько) | — |
| `--target` | Целевая папка | — |
| `--template` | Шаблон папок | `2006-01-02` |
| `--name-template` | Шаблон имён файлов | `{original}{ext}` |
| `--live-photos` | Группировать Live Photos | `true` |
| `--include-video` | Обрабатывать видео | `true` |
| `--dry-run` | Пробный прогон | `true` |
| `--use-mtime` | Fallback на дату изменения | `true` |
| `--write-exif` | Записывать дату в EXIF | `false` |
| `--notify` | Системное уведомление по завершении | `true` |
| `--full-check` | Игнорировать state | `false` |
| `--reset-state` | Удалить файл состояния перед запуском | `false` |
| `--report-format` | Формат файла-отчёта: `text` или `html` | `html` |
| `--collision-strategy` | Стратегия коллизий: `counter` или `hash` | `counter` |
| `--sort-mode` | Режим сортировки: `date` или `face` | `date` |
| `--face-model-path` | Путь к ONNX-моделям (для face-режима) | — |
| `--face-similarity` | Порог cosine similarity для кластеризации | `0.6` |
| `--face-alias` | Алиасы кластеров: `id:имя,id:имя` | — |

## Первый запуск на реальных данных

1. **Всегда начинайте с пробного прогона:**
   ```bash
   ./photo-sorter --source ~/Photos --target /Volumes/ExternalDisk --dry-run
   ```
2. **Проверьте отчёт:** убедитесь, что даты определены правильно, дубли корректны.
3. **Запустите реальное копирование:**
   ```bash
   ./photo-sorter --source ~/Photos --target /Volumes/ExternalDisk --dry-run=false
   ```
4. **Проверьте результат** — файлы скопированы, оригиналы на месте.

## Инкрементальные запуски

По умолчанию `photo-sorter` запоминает, какие файлы уже были обработаны, и при повторном запуске на те же `source → target` пропускает неизменившиеся файлы.

- Состояние хранится в `<target>/.photo-sorter/state.bolt`.
- Новые и изменённые файлы обрабатываются полностью.
- Дубликаты уже скопированных файлов распознаются и пропускаются.

```bash
# Обычный инкрементальный запуск
./photo-sorter --source ~/Photos --target ~/Sorted --dry-run=false

# Полный пересорт (игнорировать state)
./photo-sorter --source ~/Photos --target ~/Sorted --full-check --dry-run=false

# Полный сброс — удалить state и пересортировать всё с нуля
./photo-sorter --source ~/Photos --target ~/Sorted --reset-state --dry-run=false
```

> `--dry-run` не изменяет state-файл.

## Шаблоны имён файлов

По умолчанию файлы сохраняют оригинальные имена (`{original}{ext}`).

### Плейсхолдеры

| Плейсхолдер | Описание | Пример |
|-------------|----------|--------|
| `{YYYY}` | Год (4 цифры) | `2024` |
| `{YY}` | Год (2 цифры) | `24` |
| `{MM}` | Месяц | `03` |
| `{DD}` | День | `15` |
| `{HH}` | Часы | `14` |
| `{mm}` | Минуты | `30` |
| `{SS}` | Секунды | `22` |
| `{original}` | Имя файла без расширения | `IMG_1234` |
| `{ext}` | Расширение с точкой | `.jpg` |
| `{device}` | Устройство-источник | `iPhone` |
| `{seq}` | Порядковый номер в папке | `1` |
| `{seq:03}` | Порядковый номер с ведущими нулями | `001` |

### Примеры

```bash
--name-template "{YYYY}-{MM}-{DD}_{original}{ext}"
--name-template "{YYYY}-{MM}-{DD}_{HH}-{mm}-{SS}_{device}{ext}"
--name-template "{YYYY}{MM}{DD}_{seq:03}{ext}"
```

> Файлы без даты (`unsorted/`) получают `0000-00-00_00-00-00` вместо плейсхолдеров даты.

## Сортировка по лицам (face mode)

В режиме `--sort-mode=face` приложение распознаёт лица на фотографиях, группирует их по людям и раскладывает файлы по папкам вида:

```
2024/
  03-15/
    unknown_1/
    мама/
    папа/
    no_faces/
```

### CLI

```bash
# Сортировка по людям
./photo-sorter --source ~/Photos --target ~/Sorted --sort-mode=face --dry-run=false

# С собственными алиасами
./photo-sorter --source ~/Photos --target ~/Sorted --sort-mode=face \
    --face-alias="0:папа,1:мама" --dry-run=false

# Указать свой путь к ONNX-моделям (вместо ~/.photo-sorter/models/)
./photo-sorter --source ~/Photos --target ~/Sorted --sort-mode=face \
    --face-model-path=/opt/photo-sorter-models --dry-run=false
```

### TUI

На экране **Настройки** выберите «Режим сортировки: По людям».

### Требования

1. Установленный `onnxruntime` (`brew install onnxruntime` на macOS).
2. ONNX-модели скачиваются **автоматически** при первом запуске face-режима в `~/.photo-sorter/models/`. Чтобы использовать другую папку, укажите `--face-model-path`.

> Фото без лиц помещаются в подпапку `no_faces/`. Фото с несколькими лицами привязываются к наибольшему обнаруженному лицу.

## Поддерживаемые форматы

- **Фото:** `.jpg`, `.jpeg`, `.png`, `.heic`
- **Видео:** `.mov`, `.mp4`
