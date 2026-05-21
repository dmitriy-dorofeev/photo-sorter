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
// и возвращает новый срез entries, где файл с несколькими лицами
// представлен несколькими Entry (по одному на каждый alias).
func (r *Runner) ApplyClustering(ctx context.Context, entries []sorter.Entry, aliasMgr *facealias.Manager) ([]sorter.Entry, error) {
	if r.detector == nil || r.recognizer == nil {
		return nil, fmt.Errorf("face models not loaded")
	}

	// Собираем пути всех non-skip entries
	paths := make([]string, 0, len(entries))
	entryIndex := make(map[string][]int, len(entries)) // path → индексы в entries
	for i, e := range entries {
		if e.Skip {
			continue
		}
		paths = append(paths, e.Source.Path)
		entryIndex[e.Source.Path] = append(entryIndex[e.Source.Path], i)
	}

	// Кластеризуем все файлы глобально (без группировки по датам)
	aliases, err := r.clusterGroup(ctx, paths, aliasMgr)
	if err != nil {
		return nil, fmt.Errorf("face clustering: %w", err)
	}

	// Строим новый срез entries: дублируем Entry для каждого alias'а
	var result []sorter.Entry
	for i := range entries {
		if entries[i].Skip {
			result = append(result, entries[i])
			continue
		}
		path := entries[i].Source.Path
		fileAliases := aliases[path]
		if len(fileAliases) == 0 {
			// Нет лиц → папка no_faces
			base := filepath.Base(entries[i].Target)
			entries[i].Target = filepath.Join(r.cfg.TargetRoot, noFacesAlias, base)
			result = append(result, entries[i])
			continue
		}
		// Есть одно или несколько лиц → дублируем Entry для каждого alias
		for _, alias := range fileAliases {
			base := filepath.Base(entries[i].Target)
			newEntry := entries[i]
			newEntry.Target = filepath.Join(r.cfg.TargetRoot, alias, base)
			result = append(result, newEntry)
		}
	}

	return result, nil
}

// clusterGroup обрабатывает все файлы глобально (без разбивки по датам).
// Возвращает map: путь к файлу → список alias'ов (по одному на каждое уникальное лицо).
func (r *Runner) clusterGroup(ctx context.Context, paths []string, aliasMgr *facealias.Manager) (map[string][]string, error) {

	// faceInfo хранит все embedding'и для конкретного файла
	type fileFaces struct {
		path       string
		embeddings [][]float32 // embedding каждого найденного лица
	}

	var files []fileFaces
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

			// Извлекаем embedding для КАЖДОГО лица
			var embeddings [][]float32
			for _, box := range boxes {
				faceImg := facerecogn.AlignFace(img, box.Landmarks)
				emb, err := r.recognizer.Embedding(ctx, faceImg)
				if err != nil {
					continue
				}
				embeddings = append(embeddings, emb)
			}

			if len(embeddings) == 0 {
				return nil
			}

			mu.Lock()
			files = append(files, fileFaces{path: path, embeddings: embeddings})
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	result := make(map[string][]string, len(paths))

	if len(files) == 0 {
		// Нет лиц ни на одном фото — все в no_faces
		for _, p := range paths {
			result[p] = nil
		}
		return result, nil
	}

	// Кластеризация: собираем ВСЕ лица со всех фото в один пул
	allEmbeddings := make([][]float32, 0, len(files)*2)
	for _, f := range files {
		allEmbeddings = append(allEmbeddings, f.embeddings...)
	}
	clusters := facecluster.Clusterize(allEmbeddings, r.cfg.Similarity)

	// Назначаем alias каждому кластеру (глобально — date="global")
	clusterToAlias := make(map[int]string)
	clusterMembers := make(map[int][][]float32)
	embIdx := 0
	for _, f := range files {
		for range f.embeddings {
			cid := clusters[embIdx].ClusterID
			clusterMembers[cid] = append(clusterMembers[cid], allEmbeddings[embIdx])
			embIdx++
		}
	}
	for cid, members := range clusterMembers {
		alias := aliasMgr.GetAlias("global", members)
		clusterToAlias[cid] = alias
	}

	// Для каждого файла собираем уникальные alias'ы его лиц
	embIdx = 0
	for _, f := range files {
		seen := make(map[string]struct{})
		var aliases []string
		for range f.embeddings {
			cid := clusters[embIdx].ClusterID
			alias := clusterToAlias[cid]
			if _, ok := seen[alias]; !ok {
				seen[alias] = struct{}{}
				aliases = append(aliases, alias)
			}
			embIdx++
		}
		result[f.path] = aliases
	}

	// Файлы без лиц → nil (no_faces)
	for _, p := range paths {
		if _, ok := result[p]; !ok {
			result[p] = nil
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
