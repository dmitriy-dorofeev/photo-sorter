#!/bin/bash
set -euo pipefail

BINARY_PATH="${1:?Укажите путь к бинарнику}"
GOOS="${2:?Укажите GOOS}"
GOARCH="${3:?Укажите GOARCH}"
VERSION="${4:?Укажите версию}"

if [ "$GOOS" != "darwin" ]; then
    exit 0
fi

DIR_NAME=$(dirname "$BINARY_PATH")
APP_PATH=$(bash scripts/package-macos-app.sh "$BINARY_PATH" "$DIR_NAME")

# Нормализация имени архитектуры
if [ "$GOARCH" = "amd64" ]; then
    GOARCH="x86_64"
fi

ZIP_NAME="photo-sorter_${VERSION}_macOS_${GOARCH}.app.zip"
rm -f "dist/$ZIP_NAME"
(cd "$(dirname "$APP_PATH")" && zip -r --symlinks "../../dist/$ZIP_NAME" "$(basename "$APP_PATH")")
rm -rf "$APP_PATH"
echo "Created dist/$ZIP_NAME"
