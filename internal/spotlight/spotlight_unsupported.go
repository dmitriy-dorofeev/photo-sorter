//go:build !darwin

package spotlight

import "time"

func available() bool {
	return false
}

func writeTags(path string, date time.Time) error {
	return nil
}
