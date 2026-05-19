//go:build darwin

package spotlight

import (
	"fmt"
	"time"

	"golang.org/x/sys/unix"
	"howett.net/plist"
)

const (
	xattrUserTags = "com.apple.metadata:kMDItemUserTags"
	xattrComment  = "com.apple.metadata:kMDItemComment"
)

func available() bool {
	return true
}

func writeTags(path string, date time.Time) error {
	if date.IsZero() {
		return fmt.Errorf("cannot write zero date to spotlight tags")
	}

	dateStr := date.Format("2006-01-02")

	// kMDItemUserTags ожидает массив строк (plist).
	tags := []string{dateStr}
	tagsData, err := plist.Marshal(tags, plist.BinaryFormat)
	if err != nil {
		return fmt.Errorf("failed to marshal user tags plist: %w", err)
	}

	if err := unix.Setxattr(path, xattrUserTags, tagsData, 0); err != nil {
		return fmt.Errorf("failed to set kMDItemUserTags: %w", err)
	}

	// kMDItemComment ожидает строку (plist).
	commentData, err := plist.Marshal(dateStr, plist.BinaryFormat)
	if err != nil {
		// Comment не критичен, tags уже записаны.
		return nil
	}

	_ = unix.Setxattr(path, xattrComment, commentData, 0)
	return nil
}
