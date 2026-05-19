package facecluster

import (
	"math"
	"testing"
)

func TestClusterize(t *testing.T) {
	// Создаём 3 кластера + шум
	// Кластер 0: вектора близкие к [1,0,0,...]
	// Кластер 1: вектора близкие к [0,1,0,...]
	// Кластер 2: один вектор (шум)
	emb := make([][]float32, 7)
	emb[0] = l2norm([]float32{1, 0.1, 0, 0})
	emb[1] = l2norm([]float32{0.9, 0.2, 0, 0})
	emb[2] = l2norm([]float32{1.1, 0, 0, 0})
	emb[3] = l2norm([]float32{0, 1, 0.1, 0})
	emb[4] = l2norm([]float32{0.1, 0.9, 0, 0})
	emb[5] = l2norm([]float32{0, 1.1, 0, 0})
	emb[6] = l2norm([]float32{0, 0, 0, 1}) // шум, далеко от всех

	results := Clusterize(emb, 0.6)
	if len(results) != len(emb) {
		t.Fatalf("expected %d results, got %d", len(emb), len(results))
	}

	// Проверяем, что элементы 0,1,2 в одном кластере
	if results[0].ClusterID != results[1].ClusterID || results[0].ClusterID != results[2].ClusterID {
		t.Errorf("expected elements 0,1,2 in same cluster, got %d, %d, %d",
			results[0].ClusterID, results[1].ClusterID, results[2].ClusterID)
	}

	// Элементы 3,4,5 в другом кластере
	if results[3].ClusterID != results[4].ClusterID || results[3].ClusterID != results[5].ClusterID {
		t.Errorf("expected elements 3,4,5 in same cluster, got %d, %d, %d",
			results[3].ClusterID, results[4].ClusterID, results[5].ClusterID)
	}

	// Кластеры 0,1,2 и 3,4,5 разные
	if results[0].ClusterID == results[3].ClusterID {
		t.Errorf("expected different clusters for 0,1,2 and 3,4,5")
	}
}

func TestClusterize_Empty(t *testing.T) {
	results := Clusterize(nil, 0.6)
	if results != nil {
		t.Errorf("expected nil for empty input")
	}
}

func TestClusterize_Single(t *testing.T) {
	emb := [][]float32{l2norm([]float32{1, 0, 0})}
	results := Clusterize(emb, 0.6)
	if len(results) != 1 || results[0].ClusterID != 0 {
		t.Errorf("expected single cluster 0, got %v", results)
	}
}

func TestCosineSimilarity(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	if cosineSimilarity(a, b) != 0 {
		t.Errorf("orthogonal vectors should have similarity 0")
	}

	c := []float32{1, 0, 0}
	if cosineSimilarity(a, c) != 1 {
		t.Errorf("same vectors should have similarity 1")
	}
}

func l2norm(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	norm := math.Sqrt(sum)
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = x / float32(norm)
	}
	return out
}
