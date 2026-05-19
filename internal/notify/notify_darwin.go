//go:build darwin

package notify

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Available возвращает true, если в системе доступен хотя бы один способ отправки уведомлений.
func Available() bool {
	if _, err := terminalNotifierPath(); err == nil {
		return true
	}
	_, err := exec.LookPath("osascript")
	return err == nil
}

// Send отправляет уведомление через Apple Notification Center.
// При наличии terminal-notifier использует его (с иконкой приложения, если она найдена),
// иначе fallback на osascript.
func Send(title, body string) error {
	if err := sendWithTerminalNotifier(title, body); err == nil {
		return nil
	}
	return sendWithAppleScript(title, body)
}

func sendWithTerminalNotifier(title, body string) error {
	tnPath, err := terminalNotifierPath()
	if err != nil {
		return err
	}

	args := []string{
		"-title", title,
		"-message", body,
	}

	// Если запущены из .app bundle — указываем sender (бандл из build/macos/Info.plist).
	if sender := appBundleID(); sender != "" {
		args = append(args, "-sender", sender)
	}

	// Если нашли файл иконки — показываем её явно.
	if icon := appIconPath(); icon != "" {
		args = append(args, "-appIcon", icon)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, tnPath, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("terminal-notifier: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func sendWithAppleScript(title, body string) error {
	script := fmt.Sprintf(
		`display notification %q with title %q`,
		body, title,
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// #nosec G204 — аргументы формируются внутри приложения и экранируются через %q.
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("osascript: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// terminalNotifierPath возвращает путь к бинарнику terminal-notifier.
// Сначала ищет в PATH, затем распаковывает встроенный zip во временную директорию.
func terminalNotifierPath() (string, error) {
	if path, err := exec.LookPath("terminal-notifier"); err == nil {
		return path, nil
	}

	tmpDir := os.TempDir()
	appPath := filepath.Join(tmpDir, "terminal-notifier.app")
	binPath := filepath.Join(appPath, "Contents", "MacOS", "terminal-notifier")

	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	}

	if err := extractTerminalNotifier(tmpDir); err != nil {
		return "", err
	}
	return binPath, nil
}

func extractTerminalNotifier(dst string) error {
	r := bytes.NewReader(terminalNotifierZip)
	zr, err := zip.NewReader(r, int64(len(terminalNotifierZip)))
	if err != nil {
		return err
	}

	for _, f := range zr.File {
		path := filepath.Join(dst, f.Name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// appBundleID возвращает bundle ID если photo-sorter запущен из .app bundle.
func appBundleID() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return ""
	}
	// Проверяем структуру .app bundle.
	if strings.Contains(exe, ".app/Contents/MacOS/") {
		return "com.photosorter.app"
	}
	return ""
}

// appIconPath ищет файл иконки рядом с бинарником или внутри .app bundle.
func appIconPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return ""
	}
	dir := filepath.Dir(exe)

	candidates := []string{
		// Внутри .app bundle
		filepath.Join(dir, "..", "Resources", "photo-sorter.icns"),
		// Рядом с бинарником (для обычной сборки)
		filepath.Join(dir, "photo-sorter.icns"),
	}

	for _, c := range candidates {
		if p, err := filepath.Abs(c); err == nil {
			c = p
		}
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	return ""
}
