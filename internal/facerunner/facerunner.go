// Package facerunner выполняет face-кластеризацию над планом сортировки.
// Работает после sorter.BuildTree: группирует файлы по датам,
// извлекает embedding'и лиц, кластеризует и добавляет <alias> в TargetPath.
package facerunner

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"photo-sorter/internal/facealias"
	"photo-sorter/internal/facecluster"
	"photo-sorter/internal/facedetect"
	"photo-sorter/internal/facerecogn"
	"photo-sorter/internal/sorter"

	"golang.org/x/sync/errgroup"
)

const (
	noFacesAlias = "no_faces"
)

// Config описывает параметры face-кластеризации.
type Config struct {
	ModelPath   string
	Similarity  float32
	Concurrency int
}

// Runner выполняет face-кластеризацию.
type Runner struct {
	detector   *facedetect.Detector
	recognizer *facerecogn.Recognizer
	cfg        Config
}

// NewRunner создаёт face-runner, загружая модели.
func NewRunner(cfg Config) (*Runner, error) {
	if cfg.ModelPath == "" {
		return nil, fmt.Errorf("face model path is empty")
	}
	if cfg.Similarity <= 0 {
		cfg.Similarity = 0.6
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 4
	}

	detPath := filepath.Join(cfg.ModelPath, "face-detection.onnx")
	recPath := filepath.Join(cfg.ModelPath, "face-recognition.onnx")

	det, err := facedetect.NewDetector(detPath)
	if err != nil {
		return nil, fmt.Errorf("face detection model: %w", err)
	}

	rec, err := facerecogn.NewRecognizer(recPath)
	if err != nil {
		det.Close()
		return nil, fmt.Errorf("face recognition model: %w", err)
	}

	return &Runner{
		detector:   det,
		recognizer: rec,
		cfg:        cfg,
	}, nil
}

// Close освобождает ресурсы моделей.
func (r *Runner) Close() {
	if r.detector != nil {
		r.detector.Close()
	}
	if r.recognizer != nil {
		r.recognizer.Close()
	}
}

// ApplyClustering группирует entries по датам, выполняет face-кластеризацию
// и обновляет TargetPath: YYYY/MM-DD/ → YYYY/MM-DD/<alias>/.
func (r *Runner) ApplyClustering(ctx context.Context, entries []sorter.Entry, aliasMgr *facealias.Manager) error {
	if r.detector == nil || r.recognizer == nil {
		return fmt.Errorf("face models not loaded")
	}

	// Группируем entries по датовой директории
	groups := make(map[string][]int) // dir → индексы entries
	for i, e := range entries {
		if e.Skip {
			continue
		}
		dir := filepath.Dir(e.Target)
		groups[dir] = append(groups[dir], i)
	}

	// Для каждой группы запускаем кластеризацию
	var mu sync.Mutex
	aliasMap := make(map[int]string) // entry index → alias

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(r.cfg.Concurrency)

	for dir, indices := range groups {
		dir := dir
		indices := indices
		g.Go(func() error {
			// Собираем пути
			paths := make([]string, len(indices))
			for i, idx := range indices {
				paths[i] = entries[idx].Source.Path
			}
			aliases, err := r.clusterGroup(ctx, dir, paths, aliasMgr)
			if err != nil {
				return fmt.Errorf("cluster %s: %w", dir, err)
			}
			mu.Lock()
			for i, idx := range indices {
				if alias, ok := aliases[paths[i]]; ok {
					aliasMap[idx] = alias
				}
			}
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	// Обновляем TargetPath
	for i := range entries {
		if entries[i].Skip {
			continue
		}
		alias, ok := aliasMap[i]
		if !ok {
			continue
		}
		dir := filepath.Dir(entries[i].Target)
		base := filepath.Base(entries[i].Target)
		entries[i].Target = filepath.Join(dir, alias, base)
	}

	return nil
}

// clusterGroup обрабатывает одну датовую группу файлов.
func (r *Runner) clusterGroup(ctx context.Context, dir string, paths []string, aliasMgr *facealias.Manager) (map[string]string, error) {
	date := filepath.Base(dir)
	if date == sorter.UnsortedDir {
		date = "unsorted"
	}

	type faceInfo struct {
		path      string
		embedding []float32
	}

	var faces []faceInfo
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(r.cfg.Concurrency)

	for _, path := range paths {
		path := path
		g.Go(func() error {
			// Пропускаем видео и RAW
			ext := strings.ToLower(filepath.Ext(path))
			if !isImageExt(ext) {
				return nil
			}

			img, err := loadImage(path)
			if err != nil {
				return nil // пропускаем битые файлы
			}

			boxes, err := r.detector.Detect(ctx, img)
			if err != nil {
				return nil
			}
			if len(boxes) == 0 {
				return nil
			}

			// Берём доминантное лицо (самое большое)
			dominant := pickDominant(boxes)
			faceImg := cropFace(img, dominant)

			emb, err := r.recognizer.Embedding(ctx, faceImg)
			if err != nil {
				return nil
			}

			mu.Lock()
			faces = append(faces, faceInfo{path: path, embedding: emb})
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	result := make(map[string]string, len(paths))

	if len(faces) == 0 {
		// Нет лиц ни на одном фото — все в no_faces
		for _, p := range paths {
			result[p] = noFacesAlias
		}
		return result, nil
	}

	// Кластеризация
	embeddings := make([][]float32, len(faces))
	for i, f := range faces {
		embeddings[i] = f.embedding
	}
	clusters := facecluster.Clusterize(embeddings, r.cfg.Similarity)

	// Группируем по clusterID
	clusterMembers := make(map[int][]faceInfo)
	for i, c := range clusters {
		clusterMembers[c.ClusterID] = append(clusterMembers[c.ClusterID], faces[i])
	}

	// Назначаем alias каждому кластеру
	for _, members := range clusterMembers {
		clusterEmbeddings := make([][]float32, len(members))
		for i, m := range members {
			clusterEmbeddings[i] = m.embedding
		}
		alias := aliasMgr.GetAlias(date, clusterEmbeddings)
		for _, m := range members {
			result[m.path] = alias
		}
	}

	// Файлы без лиц → no_faces
	for _, p := range paths {
		if _, ok := result[p]; !ok {
			result[p] = noFacesAlias
		}
	}

	return result, nil
}

func isImageExt(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".heic", ".heif", ".webp", ".bmp", ".tiff":
		return true
	}
	return false
}

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

func pickDominant(boxes []facedetect.FaceBox) facedetect.FaceBox {
	var best facedetect.FaceBox
	var bestArea float32
	for _, b := range boxes {
		area := (b.X2 - b.X1) * (b.Y2 - b.Y1)
		if area > bestArea {
			bestArea = area
			best = b
		}
	}
	return best
}

func cropFace(img image.Image, box facedetect.FaceBox) image.Image {
	bounds := img.Bounds()
	minX := bounds.Min.X
	minY := bounds.Min.Y

	x1 := int(box.X1) + minX
	y1 := int(box.Y1) + minY
	x2 := int(box.X2) + minX
	y2 := int(box.Y2) + minY

	if x1 < bounds.Min.X {
		x1 = bounds.Min.X
	}
	if y1 < bounds.Min.Y {
		y1 = bounds.Min.Y
	}
	if x2 > bounds.Max.X {
		x2 = bounds.Max.X
	}
	if y2 > bounds.Max.Y {
		y2 = bounds.Max.Y
	}

	rect := image.Rect(x1, y1, x2, y2)
	return cropImage(img, rect)
}

func cropImage(img image.Image, rect image.Rectangle) image.Image {
	type subImage interface {
		SubImage(r image.Rectangle) image.Image
	}
	if s, ok := img.(subImage); ok {
		return s.SubImage(rect)
	}
	dst := image.NewRGBA(rect)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			dst.Set(x, y, img.At(x, y))
		}
	}
	return dst
}
