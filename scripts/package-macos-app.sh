#!/bin/bash
set -euo pipefail

BINARY_PATH="${1:?Укажите путь к бинарнику}"
OUTPUT_DIR="${2:?Укажите выходную директорию}"

APP_NAME="Photo Sorter.app"
APP_PATH="${OUTPUT_DIR}/${APP_NAME}"

rm -rf "$APP_PATH"
mkdir -p "$APP_PATH/Contents/MacOS"
mkdir -p "$APP_PATH/Contents/Resources"

cp "$BINARY_PATH" "$APP_PATH/Contents/MacOS/photo-sorter"
cp build/macos/wrapper.sh "$APP_PATH/Contents/MacOS/wrapper"
chmod +x "$APP_PATH/Contents/MacOS/wrapper"
chmod +x "$APP_PATH/Contents/MacOS/photo-sorter"
cp build/macos/Info.plist "$APP_PATH/Contents/Info.plist"

if [ -f build/macos/icon.png ]; then
    mkdir -p build/macos/icon.iconset
    sips -z 16 16 build/macos/icon.png --out build/macos/icon.iconset/icon_16x16.png >/dev/null 2>&1 || true
    sips -z 32 32 build/macos/icon.png --out build/macos/icon.iconset/icon_16x16@2x.png >/dev/null 2>&1 || true
    sips -z 32 32 build/macos/icon.png --out build/macos/icon.iconset/icon_32x32.png >/dev/null 2>&1 || true
    sips -z 64 64 build/macos/icon.png --out build/macos/icon.iconset/icon_32x32@2x.png >/dev/null 2>&1 || true
    sips -z 128 128 build/macos/icon.png --out build/macos/icon.iconset/icon_128x128.png >/dev/null 2>&1 || true
    sips -z 256 256 build/macos/icon.png --out build/macos/icon.iconset/icon_128x128@2x.png >/dev/null 2>&1 || true
    sips -z 256 256 build/macos/icon.png --out build/macos/icon.iconset/icon_256x256.png >/dev/null 2>&1 || true
    sips -z 512 512 build/macos/icon.png --out build/macos/icon.iconset/icon_256x256@2x.png >/dev/null 2>&1 || true
    sips -z 512 512 build/macos/icon.png --out build/macos/icon.iconset/icon_512x512.png >/dev/null 2>&1 || true
    sips -z 1024 1024 build/macos/icon.png --out build/macos/icon.iconset/icon_512x512@2x.png >/dev/null 2>&1 || true
    iconutil -c icns build/macos/icon.iconset -o "$APP_PATH/Contents/Resources/photo-sorter.icns" 2>/dev/null || true
    rm -rf build/macos/icon.iconset
fi

echo "$APP_PATH"
