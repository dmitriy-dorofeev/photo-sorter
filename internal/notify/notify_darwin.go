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

	// Если нашли файл иконки рядом с бинарником — показываем её явно.
	if icon := appIconPath(); icon != "" {
		args = append(args, "-appIcon", icon)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// #nosec G204 — tnPath получается из exec.LookPath или распаковки во временную директорию; args формируются внутри приложения.
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
		if !filepath.IsLocal(f.Name) {
			continue // пропускаем потенциально опасные пути (zip slip)
		}
		// #nosec G305 — проверка filepath.IsLocal выше защищает от zip slip.
		path := filepath.Join(dst, f.Name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0750); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		//nolint:gosec // G304: путь проверен через filepath.IsLocal и filepath.Join с доверенным dst.
		out, err := os.OpenFile(filepath.Clean(path), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			_ = rc.Close()
			return err
		}
		_, err = io.CopyN(out, rc, 10*1024*1024) // лимит 10 MB для terminal-notifier
		closeErr1 := rc.Close()
		closeErr2 := out.Close()
		if err != nil {
			return err
		}
		if closeErr1 != nil {
			return closeErr1
		}
		if closeErr2 != nil {
			return closeErr2
		}
	}
	return nil
}

// appIconPath ищет файл иконки рядом с бинарником.
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

	iconPath := filepath.Join(dir, "photo-sorter.icns")
	if p, err := filepath.Abs(iconPath); err == nil {
		iconPath = p
	}
	if fi, err := os.Stat(iconPath); err == nil && !fi.IsDir() {
		return iconPath
	}
	return ""
}
