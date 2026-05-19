//go:build darwin

package copier

import "golang.org/x/sys/unix"

func xattrGetImpl(path, name string) ([]byte, error) {
	size, err := unix.Getxattr(path, name, nil)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, size)
	_, err = unix.Getxattr(path, name, buf)
	return buf, err
}
