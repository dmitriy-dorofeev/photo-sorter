// Package facedetect выполняет детекцию лиц на изображениях с помощью YuNet (ONNX).
package facedetect

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"
	"sync"

	"photo-sorter/internal/onnxhelper"

	onnxruntime "github.com/shota3506/onnxruntime-purego/onnxruntime"
)

const (
	inputSize   = 640
	scoreThresh = 0.5
	nmsThresh   = 0.3
	topK        = 50
)

var strides = []int{8, 16, 32}

// FaceBox описывает обнаруженное лицо.
type FaceBox struct {
	X1, Y1, X2, Y2 float32
	Score          float32
	Landmarks      [5][2]float32 // 5 точек: правый глаз, левый глаз, нос, правый угол рта, левый угол рта
}

// Detector загружает YuNet-модель и выполняет инференс.
type Detector struct {
	sess *onnxhelper.Session
	mu   sync.Mutex
}

// NewDetector создаёт детектор из файла модели.
func NewDetector(modelPath string) (*Detector, error) {
	sess, err := onnxhelper.NewSession(modelPath)
	if err != nil {
		return nil, err
	}
	return &Detector{sess: sess}, nil
}

// Close освобождает ресурсы.
func (d *Detector) Close() {
	if d.sess != nil {
		d.sess.Close()
	}
}

// Detect находит лица на изображении.
func (d *Detector) Detect(ctx context.Context, img image.Image) ([]FaceBox, error) {
	if d.sess == nil {
		return nil, fmt.Errorf("detector not initialized")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Предобработка: resize до 640×640, RGB, float32 [0,255]
	tensorData, origW, origH := preprocess(img)

	inputShape := []int64{1, 3, inputSize, inputSize}
	inputValue, err := onnxruntime.NewTensorValue(d.sess.Runtime, tensorData, inputShape)
	if err != nil {
		return nil, fmt.Errorf("create input tensor: %w", err)
	}
	defer inputValue.Close()

	inputs := map[string]*onnxruntime.Value{"input": inputValue}
	outputs, err := d.sess.Session.Run(ctx, inputs)
	if err != nil {
		return nil, fmt.Errorf("inference: %w", err)
	}
	for _, v := range outputs {
		defer v.Close()
	}

	faces, err := postprocess(outputs, origW, origH)
	if err != nil {
		return nil, fmt.Errorf("postprocess: %w", err)
	}

	return faces, nil
}

// preprocess преобразует image.Image в NCHW float32 тензор.
// Использует letterbox (сохранение aspect ratio + паддинг до inputSize),
// как в референсной реализации OpenCV FaceDetectorYN.
func preprocess(img image.Image) (data []float32, origW, origH int) {
	bounds := img.Bounds()
	origW = bounds.Dx()
	origH = bounds.Dy()

	// Масштабируем с сохранением пропорций, чтобы поместиться в inputSize.
	// Если изображение меньше inputSize — просто центрируем без upscale.
	var newW, newH int
	if origW <= inputSize && origH <= inputSize {
		newW, newH = origW, origH
	} else {
		scale := min(float64(inputSize)/float64(origW), float64(inputSize)/float64(origH))
		newW = int(float64(origW) * scale)
		newH = int(float64(origH) * scale)
	}

	data = make([]float32, 3*inputSize*inputSize)
	for y := 0; y < inputSize; y++ {
		for x := 0; x < inputSize; x++ {
			var r, g, b float32
			if x < newW && y < newH {
				srcX := float64(x) * float64(origW) / float64(newW)
				srcY := float64(y) * float64(origH) / float64(newH)
				r, g, b = sampleRGB(img, srcX, srcY)
			}
			// padding — уже нули
			idx := y*inputSize + x
			data[0*inputSize*inputSize+idx] = r // R
			data[1*inputSize*inputSize+idx] = g // G
			data[2*inputSize*inputSize+idx] = b // B
		}
	}
	return data, origW, origH
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// sampleRGB выполняет билинейную интерполяцию пикселя.
func sampleRGB(img image.Image, x, y float64) (r, g, b float32) {
	bounds := img.Bounds()
	minX, minY := float64(bounds.Min.X), float64(bounds.Min.Y)
	maxX, maxY := float64(bounds.Max.X-1), float64(bounds.Max.Y-1)

	x0 := int(math.Floor(x + minX))
	y0 := int(math.Floor(y + minY))
	x1 := x0 + 1
	y1 := y0 + 1

	x0 = clamp(x0, int(minX), int(maxX))
	x1 = clamp(x1, int(minX), int(maxX))
	y0 = clamp(y0, int(minY), int(maxY))
	y1 = clamp(y1, int(minY), int(maxY))

	dx := float32(x + minX - float64(x0))
	dy := float32(y + minY - float64(y0))

	r00, g00, b00 := pixelRGB(img.At(x0, y0))
	r10, g10, b10 := pixelRGB(img.At(x1, y0))
	r01, g01, b01 := pixelRGB(img.At(x0, y1))
	r11, g11, b11 := pixelRGB(img.At(x1, y1))

	r = lerp(lerp(r00, r10, dx), lerp(r01, r11, dx), dy)
	g = lerp(lerp(g00, g10, dx), lerp(g01, g11, dx), dy)
	b = lerp(lerp(b00, b10, dx), lerp(b01, b11, dx), dy)
	return
}

func pixelRGB(c color.Color) (r, g, b float32) {
	r16, g16, b16, _ := c.RGBA()
	return float32(r16) / 257.0, float32(g16) / 257.0, float32(b16) / 257.0
}

func lerp(a, b, t float32) float32 {
	return a + t*(b-a)
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// postprocess декодирует выходы YuNet и применяет NMS.
func postprocess(outputs map[string]*onnxruntime.Value, origW, origH int) ([]FaceBox, error) {
	// Извлекаем float32 тензоры из outputs
	var cls [3][]float32
	var obj [3][]float32
	var bbox [3][]float32
	var kps [3][]float32

	outputNames := []string{
		"cls_8", "cls_16", "cls_32",
		"obj_8", "obj_16", "obj_32",
		"bbox_8", "bbox_16", "bbox_32",
		"kps_8", "kps_16", "kps_32",
	}

	for i, name := range outputNames {
		v, ok := outputs[name]
		if !ok {
			return nil, fmt.Errorf("output %s not found", name)
		}
		data, shape, err := onnxruntime.GetTensorData[float32](v)
		if err != nil {
			return nil, fmt.Errorf("read output %s: %w", name, err)
		}
		_ = shape // shape проверяется неявно через длину
		idx := i % 3
		switch i / 3 {
		case 0:
			cls[idx] = data
		case 1:
			obj[idx] = data
		case 2:
			bbox[idx] = data
		case 3:
			// #nosec G602: idx вычисляется как i%3 и всегда находится в диапазоне [0,2].
			kps[idx] = data
		}
	}

	scaleX := float32(origW) / float32(inputSize)
	scaleY := float32(origH) / float32(inputSize)

	var faces []FaceBox
	for i, stride := range strides {
		cols := inputSize / stride
		rows := inputSize / stride
		total := rows * cols
		kpsStride, err := kpsByStride(kps, i)
		if err != nil {
			return nil, err
		}

		if len(cls[i]) < total || len(obj[i]) < total {
			return nil, fmt.Errorf("invalid cls/obj tensor length for stride %d", stride)
		}
		if len(bbox[i]) < total*4 {
			return nil, fmt.Errorf("invalid bbox tensor length for stride %d", stride)
		}
		if len(kpsStride) < total*10 {
			return nil, fmt.Errorf("invalid kps tensor length for stride %d", stride)
		}

		for r := 0; r < rows; r++ {
			for c := 0; c < cols; c++ {
				idx := r*cols + c

				clsScore := clamp01(cls[i][idx])
				objScore := clamp01(obj[i][idx])
				score := float32(math.Sqrt(float64(clsScore * objScore)))

				if score < scoreThresh {
					continue
				}

				// Decode bbox
				cx := (float32(c) + bbox[i][idx*4+0]) * float32(stride)
				cy := (float32(r) + bbox[i][idx*4+1]) * float32(stride)
				w := float32(math.Exp(float64(bbox[i][idx*4+2]))) * float32(stride)
				h := float32(math.Exp(float64(bbox[i][idx*4+3]))) * float32(stride)

				x1 := cx - w/2
				y1 := cy - h/2
				x2 := cx + w/2
				y2 := cy + h/2

				// Scale back to original image
				x1 *= scaleX
				y1 *= scaleY
				x2 *= scaleX
				y2 *= scaleY

				var landmarks [5][2]float32
				for n := 0; n < 5; n++ {
					lxRaw, lyRaw, err := kpsPoint(kpsStride, idx, n)
					if err != nil {
						return nil, err
					}
					lx := (lxRaw + float32(c)) * float32(stride) * scaleX
					ly := (lyRaw + float32(r)) * float32(stride) * scaleY
					// #nosec G602: n ограничен циклом 0..4, landmarks имеет фиксированный размер [5].
					landmarks[n] = [2]float32{lx, ly}
				}

				faces = append(faces, FaceBox{
					X1:        x1,
					Y1:        y1,
					X2:        x2,
					Y2:        y2,
					Score:     score,
					Landmarks: landmarks,
				})
			}
		}
	}

	if len(faces) == 0 {
		return nil, nil
	}

	// NMS
	keep := nms(faces, nmsThresh, topK)
	result := make([]FaceBox, 0, len(keep))
	for _, i := range keep {
		result = append(result, faces[i])
	}
	return result, nil
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func kpsPoint(kps []float32, anchorIdx, pointIdx int) (float32, float32, error) {
	base := anchorIdx*10 + 2*pointIdx
	if base < 0 || base+1 >= len(kps) {
		return 0, 0, fmt.Errorf("invalid kps index: base=%d len=%d", base, len(kps))
	}
	return kps[base], kps[base+1], nil
}

func kpsByStride(kps [3][]float32, strideIdx int) ([]float32, error) {
	switch strideIdx {
	case 0:
		return kps[0], nil
	case 1:
		return kps[1], nil
	case 2:
		return kps[2], nil
	default:
		return nil, fmt.Errorf("invalid stride index: %d", strideIdx)
	}
}

// nms выполняет Non-Maximum Suppression.
func nms(faces []FaceBox, threshold float32, maxCount int) []int {
	type item struct {
		idx   int
		score float32
	}
	items := make([]item, len(faces))
	for i := range faces {
		items[i] = item{idx: i, score: faces[i].Score}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].score > items[j].score
	})

	var keep []int
	for len(items) > 0 && len(keep) < maxCount {
		cur := items[0]
		keep = append(keep, cur.idx)
		items = items[1:]

		var next []item
		for _, it := range items {
			if iou(faces[cur.idx], faces[it.idx]) < threshold {
				next = append(next, it)
			}
		}
		items = next
	}
	return keep
}

func iou(a, b FaceBox) float32 {
	x1 := max(a.X1, b.X1)
	y1 := max(a.Y1, b.Y1)
	x2 := min32(a.X2, b.X2)
	y2 := min32(a.Y2, b.Y2)

	inter := max(0, x2-x1) * max(0, y2-y1)
	areaA := (a.X2 - a.X1) * (a.Y2 - a.Y1)
	areaB := (b.X2 - b.X1) * (b.Y2 - b.Y1)
	union := areaA + areaB - inter
	if union <= 0 {
		return 0
	}
	return inter / union
}

func max(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
