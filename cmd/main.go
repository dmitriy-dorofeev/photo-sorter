package main

import (
	"flag"
	"fmt"
	"os"

	"photo-sorter/tui"
)

func main() {
	var useTUI bool
	flag.BoolVar(&useTUI, "tui", true, "Запустить в интерактивном TUI-режиме")
	flag.Parse()

	if useTUI {
		tui.Run()
		return
	}

	// TODO: CLI-режим с флагами --source, --target, --dry-run
	fmt.Println("photo-sorter — скелет приложения")
	fmt.Println("Использование:")
	fmt.Println("  ./photo-sorter")
	fmt.Println("  ./photo-sorter --source <папка> --target <папка> [--dry-run]")
	os.Exit(0)
}
