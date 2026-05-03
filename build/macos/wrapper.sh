#!/bin/bash
# Wrapper для запуска TUI-приложения в Terminal.app при двойном клике на .app bundle
DIR="$(cd "$(dirname "$0")" && pwd)"
osascript <<EOF
tell application "Terminal"
    do script "\"$DIR/photo-sorter\"; exit"
    activate
end tell
EOF
