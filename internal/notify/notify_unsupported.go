//go:build !darwin && !linux

package notify

import "errors"

// ErrUnsupported возвращается при попытке отправить уведомление на неподдерживаемой платформе.
var ErrUnsupported = errors.New("уведомления не поддерживаются на данной платформе")

// Available возвращает false для неподдерживаемых платформ.
func Available() bool {
	return false
}

// Send возвращает ErrUnsupported.
func Send(title, body string) error {
	return ErrUnsupported
}
