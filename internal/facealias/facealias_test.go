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

func TestManager_GetAliases(t *testing.T) {
	m := NewManager()
	// Два разных лица на одном фото
	emb1 := []float32{1, 0, 0}
	emb2 := []float32{0, 1, 0}
	aliases := m.GetAliases("global", [][]float32{emb1, emb2})
	if len(aliases) != 2 {
		t.Errorf("expected 2 aliases for 2 different faces, got %d", len(aliases))
	}
	// Те же лица ещё раз — должны получить те же alias'ы
	aliases2 := m.GetAliases("global", [][]float32{emb1, emb2})
	if len(aliases2) != 2 {
		t.Errorf("expected 2 aliases, got %d", len(aliases2))
	}
	for i := range aliases {
		if aliases[i] != aliases2[i] {
			t.Errorf("expected same alias at %d, got %s vs %s", i, aliases[i], aliases2[i])
		}
	}
	// Два одинаковых лица на фото — должен быть 1 alias
	aliases3 := m.GetAliases("global", [][]float32{emb1, emb1})
	if len(aliases3) != 1 {
		t.Errorf("expected 1 alias for identical faces, got %d", len(aliases3))
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
