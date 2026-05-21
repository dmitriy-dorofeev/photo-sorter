// Package facealias управляет именами (alias) для кластеров лиц.
package facealias

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/cespare/xxhash/v2"
)

const unknownPrefix = "unknown"

// Manager хранит mapping кластер → alias.
type Manager struct {
	mu      sync.RWMutex
	aliases map[string]string // ключ: date+"|"+centroidHash → значение: alias
	counter int
}

// NewManager создаёт новый менеджер alias'ов.
func NewManager() *Manager {
	return &Manager{
		aliases: make(map[string]string),
		counter: 0,
	}
}

// SetAlias задаёт имя для кластера в конкретную дату.
func (m *Manager) SetAlias(date string, embeddings [][]float32, alias string) {
	key := makeKey(date, embeddings)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.aliases[key] = alias
}

// GetAlias возвращает имя кластера. Если не задано — генерирует unknown_N.
func (m *Manager) GetAlias(date string, embeddings [][]float32) string {
	key := makeKey(date, embeddings)
	m.mu.RLock()
	if alias, ok := m.aliases[key]; ok {
		m.mu.RUnlock()
		return alias
	}
	m.mu.RUnlock()

	// Генерируем новый unknown_N
	m.mu.Lock()
	defer m.mu.Unlock()
	if alias, ok := m.aliases[key]; ok {
		return alias // double-check
	}
	m.counter++
	alias := fmt.Sprintf("%s_%d", unknownPrefix, m.counter)
	m.aliases[key] = alias
	return alias
}

// GetAliases возвращает уникальные alias'ы для набора embedding'ов (например, все лица на одном фото).
// Каждый embedding кластеризуется отдельно; результат дедуплицируется.
func (m *Manager) GetAliases(date string, embeddings [][]float32) []string {
	seen := make(map[string]struct{})
	var res []string
	for _, emb := range embeddings {
		// Одиночный embedding оборачиваем в срез для совместимости с makeKey
		single := [][]float32{emb}
		alias := m.GetAlias(date, single)
		if _, ok := seen[alias]; !ok {
			seen[alias] = struct{}{}
			res = append(res, alias)
		}
	}
	return res
}

// GetAliasByKey возвращает alias по предвычисленному ключу.
func (m *Manager) GetAliasByKey(key string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.aliases[key]
}

// SetAliasByKey задаёт alias по ключу.
func (m *Manager) SetAliasByKey(key, alias string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.aliases[key] = alias
}

// AllKeys возвращает все ключи и alias'ы.
func (m *Manager) AllKeys() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]string, len(m.aliases))
	for k, v := range m.aliases {
		out[k] = v
	}
	return out
}

// LoadFromMap загружает alias'ы из map (например, из state).
func (m *Manager) LoadFromMap(data map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range data {
		m.aliases[k] = v
		// Обновляем counter для unknown_N
		if strings.HasPrefix(v, unknownPrefix+"_") {
			n, err := strconv.Atoi(strings.TrimPrefix(v, unknownPrefix+"_"))
			if err == nil && n > m.counter {
				m.counter = n
			}
		}
	}
}

// makeKey создаёт ключ для кластера по дате и centroid embedding'ов.
func makeKey(date string, embeddings [][]float32) string {
	centroid := computeCentroid(embeddings)
	h := xxhash.Sum64(floatsToBytes(centroid))
	return fmt.Sprintf("%s|%016x", date, h)
}

// ComputeKey вычисляет ключ для внешнего использования (например, в TUI).
func ComputeKey(date string, embeddings [][]float32) string {
	return makeKey(date, embeddings)
}

// computeCentroid вычисляет средний вектор кластера.
func computeCentroid(embeddings [][]float32) []float32 {
	if len(embeddings) == 0 {
		return nil
	}
	dim := len(embeddings[0])
	centroid := make([]float32, dim)
	for _, emb := range embeddings {
		for i := range emb {
			centroid[i] += emb[i]
		}
	}
	for i := range centroid {
		centroid[i] /= float32(len(embeddings))
	}
	// L2 normalize
	var sum float64
	for _, v := range centroid {
		sum += float64(v) * float64(v)
	}
	norm := float32(math.Sqrt(sum))
	if norm > 1e-6 {
		for i := range centroid {
			centroid[i] /= norm
		}
	}
	return centroid
}

func floatsToBytes(f []float32) []byte {
	b := make([]byte, len(f)*4)
	for i, v := range f {
		bits := math.Float32bits(v)
		b[i*4+0] = byte(bits)
		b[i*4+1] = byte(bits >> 8)
		b[i*4+2] = byte(bits >> 16)
		b[i*4+3] = byte(bits >> 24)
	}
	return b
}
