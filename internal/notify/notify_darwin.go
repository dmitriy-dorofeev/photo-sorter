//go:build darwin

package notify

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Available возвращает true, если в системе доступен osascript.
func Available() bool {
	_, err := exec.LookPath("osascript")
	return err == nil
}

// Send отправляет уведомление через AppleScript (Notification Center).
func Send(title, body string) error {
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
