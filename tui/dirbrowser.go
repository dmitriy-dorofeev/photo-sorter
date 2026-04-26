package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// dirItem описывает один элемент в списке папок.
type dirItem struct {
	name     string
	path     string
	isParent bool // true для ".."
}

// dirBrowserModel — обобщённая модель браузера папок.
// Используется в экранах выбора источника и цели.
type dirBrowserModel struct {
	currentDir string
	items      []dirItem
	cursor     int
	width      int
	height     int
}

func newDirBrowserModel(startDir string) dirBrowserModel {
	return dirBrowserModel{
		currentDir: startDir,
		items:      loadDirItems(startDir),
		cursor:     0,
	}
}

// loadDirItems возвращает список папок в директории + элемент "..".
func loadDirItems(dir string) []dirItem {
	var items []dirItem

	clean := filepath.Clean(dir)
	if clean != "/" {
		items = append(items, dirItem{
			name:     "..",
			path:     filepath.Dir(clean),
			isParent: true,
		})
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return items
	}

	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			items = append(items, dirItem{
				name: e.Name(),
				path: filepath.Join(dir, e.Name()),
			})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].isParent && !items[j].isParent {
			return true
		}
		if !items[i].isParent && items[j].isParent {
			return false
		}
		return items[i].name < items[j].name
	})

	return items
}

func (db *dirBrowserModel) moveUp() {
	if db.cursor > 0 {
		db.cursor--
	}
}

func (db *dirBrowserModel) moveDown() {
	if db.cursor < len(db.items)-1 {
		db.cursor++
	}
}

func (db *dirBrowserModel) enter() {
	if len(db.items) == 0 {
		return
	}
	item := db.items[db.cursor]
	db.currentDir = item.path
	db.items = loadDirItems(item.path)
	db.cursor = 0
}

func (db *dirBrowserModel) goBack() {
	parent := filepath.Dir(filepath.Clean(db.currentDir))
	if parent != db.currentDir {
		db.currentDir = parent
		db.items = loadDirItems(parent)
		db.cursor = 0
	}
}

func (db *dirBrowserModel) selectedItem() (dirItem, bool) {
	if len(db.items) == 0 {
		return dirItem{}, false
	}
	return db.items[db.cursor], true
}
