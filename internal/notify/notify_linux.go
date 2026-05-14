//go:build linux

package notify

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Available возвращает true, если в системе доступен notify-send.
func Available() bool {
	_, err := exec.LookPath("notify-send")
	return err == nil
}

// Send отправляет уведомление через notify-send (libnotify / freedesktop).
func Send(title, body string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "notify-send", title, body)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("notify-send: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
