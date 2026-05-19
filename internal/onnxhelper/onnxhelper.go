// Package onnxhelper предоставляет упрощённую обёртку над ONNX Runtime
// для загрузки моделей и выполнения инференса.
package onnxhelper

import (
	"fmt"
	"io"
	"os"
	"sync"

	onnxruntime "github.com/shota3506/onnxruntime-purego/onnxruntime"
)

var (
	once      sync.Once
	rt        *onnxruntime.Runtime
	rtErr     error
	rtLibPath string
)

// SetLibPath задаёт путь к shared library ONNX Runtime (libonnxruntime.dylib/.so).
// Если пусто — используется стандартный поиск системы + типичные пути Homebrew.
func SetLibPath(path string) {
	rtLibPath = path
}

// findLibPath ищет libonnxruntime в типичных системных путях.
func findLibPath() string {
	candidates := []string{
		"/opt/homebrew/lib/libonnxruntime.dylib",
		"/usr/local/lib/libonnxruntime.dylib",
		"/usr/lib/libonnxruntime.dylib",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// Runtime возвращает глобальный экземпляр ONNX Runtime (singleton).
func Runtime() (*onnxruntime.Runtime, error) {
	once.Do(func() {
		libPath := rtLibPath
		if libPath == "" {
			libPath = findLibPath()
		}
		rt, rtErr = onnxruntime.NewRuntime(libPath, 23)
	})
	if rtErr != nil {
		return nil, rtErr
	}
	return rt, nil
}

// Session загружает ONNX-модель из файла и создаёт сессию инференса.
type Session struct {
	Runtime *onnxruntime.Runtime
	Env     *onnxruntime.Env
	Session *onnxruntime.Session
}

// NewSession создаёт сессию из файла модели.
func NewSession(modelPath string) (*Session, error) {
	r, err := Runtime()
	if err != nil {
		return nil, fmt.Errorf("onnx runtime: %w", err)
	}

	env, err := r.NewEnv("photo-sorter", onnxruntime.LoggingLevelWarning)
	if err != nil {
		return nil, fmt.Errorf("onnx env: %w", err)
	}

	sess, err := r.NewSession(env, modelPath, nil)
	if err != nil {
		env.Close()
		return nil, fmt.Errorf("onnx session: %w", err)
	}

	return &Session{Runtime: r, Env: env, Session: sess}, nil
}

// NewSessionFromBytes создаёт сессию из байт модели.
func NewSessionFromBytes(modelData []byte) (*Session, error) {
	r, err := Runtime()
	if err != nil {
		return nil, fmt.Errorf("onnx runtime: %w", err)
	}

	env, err := r.NewEnv("photo-sorter", onnxruntime.LoggingLevelWarning)
	if err != nil {
		return nil, fmt.Errorf("onnx env: %w", err)
	}

	sess, err := r.NewSessionFromReader(env, &bytesReader{data: modelData}, nil)
	if err != nil {
		env.Close()
		return nil, fmt.Errorf("onnx session: %w", err)
	}

	return &Session{Runtime: r, Env: env, Session: sess}, nil
}

// Close освобождает ресурсы сессии и окружения.
func (s *Session) Close() {
	if s.Session != nil {
		s.Session.Close()
	}
	if s.Env != nil {
		s.Env.Close()
	}
}

// bytesReader — простая реализация io.Reader для []byte.
type bytesReader struct {
	data []byte
	pos  int
}

func (r *bytesReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	if r.pos >= len(r.data) {
		return n, io.EOF
	}
	return n, nil
}
