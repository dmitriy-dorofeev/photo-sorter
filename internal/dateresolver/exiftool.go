package dateresolver

import "os/exec"

// FindExifTool ищет бинарник exiftool в PATH.
// Возвращает полный путь и true, если найден.
func FindExifTool() (string, bool) {
	path, err := exec.LookPath("exiftool")
	if err != nil {
		return "", false
	}
	return path, true
}
