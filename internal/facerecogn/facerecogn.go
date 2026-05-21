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

// arcfaceRefPoints — эталонные 5-точечные landmarks для ArcFace (112×112).
// Порядок: правый глаз, левый глаз, нос, правый угол рта, левый угол рта.
var arcfaceRefPoints = [5][2]float32{
	{38.2946, 51.6963},
	{73.5318, 51.5014},
	{54.0252, 71.7366},
	{39.5493, 92.3655},
	{70.7299, 92.2041},
}

// AlignFace выполняет affine alignment лица по 5 landmarks для ArcFace.
// Возвращает выровненное изображение 112×112.
func AlignFace(img image.Image, landmarks [5][2]float32) image.Image {
	// Используем 3 точки (глаза + нос) для affine transform.
	// Это даёт масштаб, поворот и сдвиг.
	var src, dst [3][2]float32
	for i := 0; i < 3; i++ {
		src[i] = landmarks[i]
		dst[i] = arcfaceRefPoints[i]
	}

	// Решаем affine transform:
	// u = a*x + b*y + c
	// v = d*x + e*y + f
	x1, y1 := src[0][0], src[0][1]
	x2, y2 := src[1][0], src[1][1]
	x3, y3 := src[2][0], src[2][1]

	u1, v1 := dst[0][0], dst[0][1]
	u2, v2 := dst[1][0], dst[1][1]
	u3, v3 := dst[2][0], dst[2][1]

	detA := x1*(y2-y3) - y1*(x2-x3) + (x2*y3 - x3*y2)
	if abs(detA) < 1e-6 {
		// Вырожденный случай — fallback на center crop
		return fallbackCrop(img)
	}

	a := (u1*(y2-y3) - y1*(u2-u3) + (u2*y3 - u3*y2)) / detA
	b := (x1*(u2-u3) - u1*(x2-x3) + (x2*u3 - x3*u2)) / detA
	c := (x1*(y2*u3-y3*u2) - y1*(x2*u3-x3*u2) + (x2*y3-x3*y2)*u1) / detA

	d := (v1*(y2-y3) - y1*(v2-v3) + (v2*y3 - v3*y2)) / detA
	e := (x1*(v2-v3) - v1*(x2-x3) + (x2*v3 - x3*v2)) / detA
	f := (x1*(y2*v3-y3*v2) - y1*(x2*v3-x3*v2) + (x2*y3-x3*y2)*v1) / detA

	// Forward matrix M:
	// | a  b  c |
	// | d  e  f |
	// | 0  0  1 |
	//
	// Inverse matrix M^-1:
	det := a*e - b*d
	if abs(det) < 1e-6 {
		return fallbackCrop(img)
	}

	inv00 := e / det
	inv01 := -b / det
	inv02 := (b*f - e*c) / det
	inv10 := -d / det
	inv11 := a / det
	inv12 := (d*c - a*f) / det

	bounds := img.Bounds()
	minX := float64(bounds.Min.X)
	minY := float64(bounds.Min.Y)
	maxX := float64(bounds.Max.X - 1)
	maxY := float64(bounds.Max.Y - 1)

	dstImg := image.NewRGBA(image.Rect(0, 0, inputSize, inputSize))

	for y := 0; y < inputSize; y++ {
		for x := 0; x < inputSize; x++ {
			srcX := inv00*float32(x) + inv01*float32(y) + inv02
			srcY := inv10*float32(x) + inv11*float32(y) + inv12

			sx := float64(srcX)
			sy := float64(srcY)

			if sx < minX || sy < minY || sx > maxX || sy > maxY {
				continue
			}

			dstImg.Set(x, y, sampleColor(img, sx, sy))
		}
	}

	return dstImg
}

// fallbackCrop — центральный квадратный crop + resize до 112×112.
func fallbackCrop(img image.Image) image.Image {
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
	return cropImage(img, rect)
}

// sampleColor выполняет билинейную интерполяцию цвета.
func sampleColor(img image.Image, x, y float64) color.Color {
	bounds := img.Bounds()
	minX, minY := float64(bounds.Min.X), float64(bounds.Min.Y)
	maxX, maxY := float64(bounds.Max.X-1), float64(bounds.Max.Y-1)

	x0 := int(math.Floor(x))
	y0 := int(math.Floor(y))
	x1 := x0 + 1
	y1 := y0 + 1

	x0 = clamp(x0, int(minX), int(maxX))
	x1 = clamp(x1, int(minX), int(maxX))
	y0 = clamp(y0, int(minY), int(maxY))
	y1 = clamp(y1, int(minY), int(maxY))

	dx := float32(x - float64(x0))
	dy := float32(y - float64(y0))

	c00 := pixelRGBA(img.At(x0, y0))
	c10 := pixelRGBA(img.At(x1, y0))
	c01 := pixelRGBA(img.At(x0, y1))
	c11 := pixelRGBA(img.At(x1, y1))

	r := lerp(lerp(c00.r, c10.r, dx), lerp(c01.r, c11.r, dx), dy)
	g := lerp(lerp(c00.g, c10.g, dx), lerp(c01.g, c11.g, dx), dy)
	b := lerp(lerp(c00.b, c10.b, dx), lerp(c01.b, c11.b, dx), dy)
	a := lerp(lerp(c00.a, c10.a, dx), lerp(c01.a, c11.a, dx), dy)

	return color.RGBA{
		R: clampUint8(int(r)),
		G: clampUint8(int(g)),
		B: clampUint8(int(b)),
		A: clampUint8(int(a)),
	}
}

type rgba struct{ r, g, b, a float32 }

func pixelRGBA(c color.Color) rgba {
	r16, g16, b16, a16 := c.RGBA()
	return rgba{
		r: float32(r16) / 257.0,
		g: float32(g16) / 257.0,
		b: float32(b16) / 257.0,
		a: float32(a16) / 257.0,
	}
}

func clampUint8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
