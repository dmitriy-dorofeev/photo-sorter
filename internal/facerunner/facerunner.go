// Package facerunner выполняет face-кластеризацию над планом сортировки.
// Работает после sorter.BuildTree: группирует файлы по датам,
// извлекает embedding'и лиц, кластеризует и добавляет <alias> в TargetPath.
package facerunner

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	TargetRoot  string // абсолютный путь к целевой папке
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

// ApplyClustering выполняет face-кластеризацию над всеми entries
// и обновляет TargetPath: <alias>/<filename> (или no_faces/<filename>).
func (r *Runner) ApplyClustering(ctx context.Context, entries []sorter.Entry, aliasMgr *facealias.Manager) error {
	if r.detector == nil || r.recognizer == nil {
		return fmt.Errorf("face models not loaded")
	}

	// Собираем пути всех non-skip entries
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Skip {
			continue
		}
		paths = append(paths, e.Source.Path)
	}

	// Кластеризуем все файлы глобально (без группировки по датам)
	aliases, err := r.clusterGroup(ctx, paths, aliasMgr)
	if err != nil {
		return fmt.Errorf("face clustering: %w", err)
	}

	// Обновляем TargetPath: полностью заменяем на <alias>/<basename>
	for i := range entries {
		if entries[i].Skip {
			continue
		}
		alias := aliases[entries[i].Source.Path]
		if alias == "" {
			alias = noFacesAlias
		}
		base := filepath.Base(entries[i].Target)
		entries[i].Target = filepath.Join(r.cfg.TargetRoot, alias, base)
	}

	return nil
}

// clusterGroup обрабатывает все файлы глобально (без разбивки по датам).
func (r *Runner) clusterGroup(ctx context.Context, paths []string, aliasMgr *facealias.Manager) (map[string]string, error) {

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
		alias := aliasMgr.GetAlias("global", clusterEmbeddings)
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
	// #nosec G304 — path приходит из scanner.FileInfo, проверенного при обходе файловой системы.
	cleanPath := filepath.Clean(path)
	f, err := os.Open(cleanPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, format, err := image.Decode(f)
	if err == nil {
		return img, nil
	}
	// Если формат неизвестен и это HEIC/HEIF — пробуем конвертировать через sips (macOS)
	if isHEICExt(filepath.Ext(path)) && runtime.GOOS == "darwin" {
		return decodeHEIC(cleanPath)
	}
	_ = format // игнорируем формат при ошибке
	return nil, err
}

func isHEICExt(ext string) bool {
	ext = strings.ToLower(ext)
	return ext == ".heic" || ext == ".heif"
}

// decodeHEIC конвертирует HEIC/HEIF во временный JPEG через sips (macOS) и декодирует его.
func decodeHEIC(path string) (image.Image, error) {
	tmpFile := path + ".tmp.jpg"
	cmd := exec.Command("sips", "-s", "format", "jpeg", path, "--out", tmpFile)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("sips convert: %w", err)
	}
	defer os.Remove(tmpFile)

	f, err := os.Open(tmpFile)
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
