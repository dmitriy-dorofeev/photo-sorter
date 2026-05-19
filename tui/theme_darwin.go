//go:build darwin

package tui

import (
	"os/exec"
	"strings"
)

// systemThemeMode определяет системную тему macOS через defaults.
func systemThemeMode() ThemeMode {
	out, err := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle").Output()
	if err != nil {
		return ThemeModeLight
	}
	if strings.TrimSpace(string(out)) == "Dark" {
		return ThemeModeDark
	}
	return ThemeModeLight
}
