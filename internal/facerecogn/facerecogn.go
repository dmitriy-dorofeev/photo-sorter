// Package facerecogn выполняет распознавание лиц (extraction embeddings) с помощью ArcFace MobileFaceNet (ONNX).
package facerecogn

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"sync"

	"photo-sorter/internal/onnxhelper"

	onnxruntime "github.com/shota3506/onnxruntime-purego/onnxruntime"
)

const inputSize = 112

// Recognizer загружает ArcFace-модель и извлекает embedding'и.
type Recognizer struct {
	sess *onnxhelper.Session
	mu   sync.Mutex
}

// NewRecognizer создаёт распознаватель из файла модели.
func NewRecognizer(modelPath string) (*Recognizer, error) {
	sess, err := onnxhelper.NewSession(modelPath)
	if err != nil {
		return nil, err
	}
	return &Recognizer{sess: sess}, nil
}

// Close освобождает ресурсы.
func (r *Recognizer) Close() {
	if r.sess != nil {
		r.sess.Close()
	}
}

// Embedding возвращает 512-мерный L2-нормализованный вектор лица.
func (r *Recognizer) Embedding(ctx context.Context, img image.Image) ([]float32, error) {
	if r.sess == nil {
		return nil, fmt.Errorf("recognizer not initialized")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	tensorData := preprocess(img)
	inputShape := []int64{1, 3, inputSize, inputSize}
	inputValue, err := onnxruntime.NewTensorValue(r.sess.Runtime, tensorData, inputShape)
	if err != nil {
		return nil, fmt.Errorf("create input tensor: %w", err)
	}
	defer inputValue.Close()

	inputs := map[string]*onnxruntime.Value{"input.1": inputValue}
	outputs, err := r.sess.Session.Run(ctx, inputs)
	if err != nil {
		return nil, fmt.Errorf("inference: %w", err)
	}
	for _, v := range outputs {
		defer v.Close()
	}

	out, ok := outputs["516"]
	if !ok {
		return nil, fmt.Errorf("output 516 not found")
	}
	data, _, err := onnxruntime.GetTensorData[float32](out)
	if err != nil {
		return nil, fmt.Errorf("read output: %w", err)
	}

	// L2 нормализация
	emb := make([]float32, 512)
	copy(emb, data)
	normalizeL2(emb)
	return emb, nil
}

// preprocess преобразует image.Image в NCHW float32 тензор.
// Нормализация: (pixel - 127.5) / 127.5 → [-1, 1]
func preprocess(img image.Image) []float32 {
	// Crop to square center
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	var rect image.Rectangle
	if w > h {
		delta := (w - h) / 2
		rect = image.Rect(bounds.Min.X+delta, bounds.Min.Y, bounds.Max.X-delta, bounds.Max.Y)
	} else {
		delta := (h - w) / 2
		rect = image.Rect(bounds.Min.X, bounds.Min.Y+delta, bounds.Max.X, bounds.Max.Y-delta)
	}
	img = cropImage(img, rect)

	data := make([]float32, 3*inputSize*inputSize)
	for y := 0; y < inputSize; y++ {
		for x := 0; x < inputSize; x++ {
			srcX := float64(x) * float64(rect.Dx()) / float64(inputSize)
			srcY := float64(y) * float64(rect.Dy()) / float64(inputSize)
			r, g, b := sampleRGB(img, srcX, srcY)
			idx := y*inputSize + x
			data[0*inputSize*inputSize+idx] = (r - 127.5) / 127.5
			data[1*inputSize*inputSize+idx] = (g - 127.5) / 127.5
			data[2*inputSize*inputSize+idx] = (b - 127.5) / 127.5
		}
	}
	return data
}

func cropImage(img image.Image, rect image.Rectangle) image.Image {
	type subImage interface {
		SubImage(r image.Rectangle) image.Image
	}
	if s, ok := img.(subImage); ok {
		return s.SubImage(rect)
	}
	// Fallback: draw manually
	dst := image.NewRGBA(rect)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			dst.Set(x, y, img.At(x, y))
		}
	}
	return dst
}

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

func normalizeL2(v []float32) {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	norm := float32(math.Sqrt(sum))
	if norm < 1e-6 {
		return
	}
	for i := range v {
		v[i] /= norm
	}
}
