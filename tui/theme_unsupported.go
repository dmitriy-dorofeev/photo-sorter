//go:build !darwin

package tui

// systemThemeMode на не-Darwin платформах возвращает светлую тему по умолчанию.
func systemThemeMode() ThemeMode {
	return ThemeModeLight
}
