#!/bin/bash
set -euo pipefail

# Wrapper для запуска TUI-приложения в Terminal.app при двойном клике на .app bundle.
# Остаётся активным, пока работает photo-sorter, чтобы иконка .app отображалась в Dock.
# После завершения TUI закрывает вкладку Terminal (или окно, если вкладка одна).

DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY="$DIR/photo-sorter"

# Запускаем Terminal и запоминаем TTY вкладки
TTY=$(osascript <<EOF
tell application "Terminal"
    set newTab to do script "\"$BINARY\"; exit"
    activate
    return tty of newTab
end tell
EOF
)

# Ждём завершения photo-sorter, чтобы .app оставался в Dock
while pgrep -f "$BINARY" > /dev/null 2>&1; do
    sleep 0.5
done

# Даём Terminal время на выполнение "exit" после завершения photo-sorter
sleep 0.5

# Закрываем вкладку Terminal, если она ещё открыта
osascript <<EOF
tell application "Terminal"
    try
        set targetTab to first tab whose tty is "$TTY"
        set targetWin to first window whose tabs contains targetTab
        if (count of tabs of targetWin) > 1 then
            close targetTab
        else
            close targetWin
        end if
    on error
        -- Уже закрыто пользователем
    end try
end tell
EOF
