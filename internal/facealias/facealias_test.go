package facealias

import (
	"strings"
	"testing"
)

func TestManager_GetAlias_GeneratesUnknown(t *testing.T) {
	m := NewManager()
	emb := [][]float32{{1, 0, 0}, {0.9, 0.1, 0}}
	alias1 := m.GetAlias("2024-03-15", emb)
	if !strings.HasPrefix(alias1, unknownPrefix+"_") {
		t.Errorf("expected unknown_N prefix, got %s", alias1)
	}

	// Тот же кластер — тот же alias
	alias2 := m.GetAlias("2024-03-15", emb)
	if alias1 != alias2 {
		t.Errorf("expected same alias for same cluster, got %s vs %s", alias1, alias2)
	}

	// Другой кластер — другой alias
	emb2 := [][]float32{{0, 1, 0}}
	alias3 := m.GetAlias("2024-03-15", emb2)
	if alias1 == alias3 {
		t.Errorf("expected different alias for different cluster")
	}
}

func TestManager_SetAlias(t *testing.T) {
	m := NewManager()
	emb := [][]float32{{1, 0, 0}}
	m.SetAlias("2024-03-15", emb, "папа")
	alias := m.GetAlias("2024-03-15", emb)
	if alias != "папа" {
		t.Errorf("expected 'папа', got %s", alias)
	}
}

func TestManager_LoadFromMap(t *testing.T) {
	m := NewManager()
	data := map[string]string{
		"2024-03-15|abc123": "мама",
		"2024-03-16|def456": unknownPrefix + "_5",
	}
	m.LoadFromMap(data)
	if m.GetAliasByKey("2024-03-15|abc123") != "мама" {
		t.Error("expected 'мама'")
	}
	if m.GetAliasByKey("2024-03-16|def456") != unknownPrefix+"_5" {
		t.Error("expected unknown_5")
	}
}
