# macOS .app bundle

## Иконка приложения

Чтобы у `.app` bundle была иконка в Finder:

1. Подготовьте PNG-изображение размером **1024×1024** пикселей.
2. Сохраните его как `build/macos/icon.png`.
3. Соберите проект командой:
   ```bash
   make build-mac-app
   ```

При сборке `icon.png` автоматически конвертируется в `.icns` и вкладывается в bundle.

Если `icon.png` отсутствует, bundle соберётся без иконки — в консоль будет выведено предупреждение.

## Автоматическая упаковка

При сборке через GoReleaser (`make snapshot` или релиз на GitHub) для macOS
автоматически создаётся `.app.zip` в директории `dist/`:

- `photo-sorter_<version>_macOS_arm64.app.zip`
- `photo-sorter_<version>_macOS_x86_64.app.zip`

Эти архивы прикрепляются к GitHub Releases как дополнительные артефакты.
