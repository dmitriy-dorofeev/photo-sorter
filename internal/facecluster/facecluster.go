// Package facecluster группирует лица (embedding-векторы) в кластеры
// с помощью графового алгоритма Chinese Whispers.
package facecluster

import (
	"math"
	"math/rand"
	"sort"
	"time"
)

// ClusterResult содержит результат кластеризации для одного файла.
type ClusterResult struct {
	ClusterID int
	Score     float32 // уверенность (среднее similarity внутри кластера)
}

// Clusterize выполняет кластеризацию embedding-векторов.
// Порог similarity: если cosine similarity ≥ threshold — создаём ребро.
func Clusterize(embeddings [][]float32, threshold float32) []ClusterResult {
	n := len(embeddings)
	if n == 0 {
		return nil
	}
	if n == 1 {
		return []ClusterResult{{ClusterID: 0, Score: 1.0}}
	}

	// Строим граф: ребро между узлами с similarity ≥ threshold
	adj := make([][]int, n)
	weights := make([][]float32, n)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			sim := cosineSimilarity(embeddings[i], embeddings[j])
			if sim >= threshold {
				adj[i] = append(adj[i], j)
				adj[j] = append(adj[j], i)
				weights[i] = append(weights[i], sim)
				weights[j] = append(weights[j], sim)
			}
		}
	}

	// Chinese Whispers
	labels := make([]int, n)
	for i := range labels {
		labels[i] = i
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Несколько итераций
	for iter := 0; iter < 20; iter++ {
		// Перемешиваем порядок узлов
		order := rng.Perm(n)
		for _, v := range order {
			if len(adj[v]) == 0 {
				continue
			}
			// Считаем веса соседних меток
			labelWeight := make(map[int]float32)
			for i, u := range adj[v] {
				labelWeight[labels[u]] += weights[v][i]
			}
			// Выбираем метку с максимальным весом
			bestLabel := labels[v]
			bestWeight := float32(0)
			for lbl, w := range labelWeight {
				if w > bestWeight {
					bestWeight = w
					bestLabel = lbl
				}
			}
			labels[v] = bestLabel
		}
	}

	// Перенумеровываем метки подряд
	labelMap := make(map[int]int)
	nextID := 0
	for i := range labels {
		if _, ok := labelMap[labels[i]]; !ok {
			labelMap[labels[i]] = nextID
			nextID++
		}
		labels[i] = labelMap[labels[i]]
	}

	// Вычисляем score для каждого кластера
	result := make([]ClusterResult, n)
	for i := 0; i < n; i++ {
		var sumSim float32
		var count int
		for j, u := range adj[i] {
			if labels[u] == labels[i] {
				sumSim += weights[i][j]
				count++
			}
		}
		score := float32(1.0)
		if count > 0 {
			score = sumSim / float32(count)
		}
		result[i] = ClusterResult{ClusterID: labels[i], Score: score}
	}

	return result
}

// cosineSimilarity вычисляет косинусное сходство двух векторов.
// Векторы должны быть L2-нормализованы (тогда dot product = cosine similarity).
func cosineSimilarity(a, b []float32) float32 {
	var sum float64
	for i := range a {
		sum += float64(a[i]) * float64(b[i])
	}
	return float32(sum)
}

// ClusterStats содержит статистику по кластерам.
type ClusterStats struct {
	ID    int
	Count int
}

// Stats возвращает отсортированную статистику по кластерам.
func Stats(results []ClusterResult) []ClusterStats {
	counts := make(map[int]int)
	for _, r := range results {
		counts[r.ClusterID]++
	}
	var stats []ClusterStats
	for id, c := range counts {
		stats = append(stats, ClusterStats{ID: id, Count: c})
	}
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Count > stats[j].Count
	})
	return stats
}

// DominantCluster возвращает ID кластера с наибольшим числом элементов.
// Если results пуст — возвращает -1.
func DominantCluster(results []ClusterResult) int {
	if len(results) == 0 {
		return -1
	}
	counts := make(map[int]int)
	for _, r := range results {
		counts[r.ClusterID]++
	}
	bestID, bestCount := -1, 0
	for id, c := range counts {
		if c > bestCount {
			bestCount = c
			bestID = id
		}
	}
	return bestID
}

// Distance вычисляет евклидово расстояние между двумя embedding'ами.
func Distance(a, b []float32) float32 {
	var sum float64
	for i := range a {
		d := float64(a[i] - b[i])
		sum += d * d
	}
	return float32(math.Sqrt(sum))
}
