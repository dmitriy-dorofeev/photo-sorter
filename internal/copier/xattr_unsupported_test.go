//go:build !darwin

package copier

func xattrGetImpl(path, name string) ([]byte, error) {
	return nil, nil
}
