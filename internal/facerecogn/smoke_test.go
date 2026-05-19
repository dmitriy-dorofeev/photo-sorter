package facerecogn

import (
	"context"
	"image"
	"math"
	"testing"
)

func TestRecognizerSmoke(t *testing.T) {
	rec, err := NewRecognizer("../../models/face-recognition.onnx")
	if err != nil {
		t.Skipf("ONNX model not available: %v", err)
	}
	defer rec.Close()

	// Создаём пустое изображение 112x112
	img := image.NewRGBA(image.Rect(0, 0, 112, 112))

	emb, err := rec.Embedding(context.Background(), img)
	if err != nil {
		t.Fatalf("embedding: %v", err)
	}
	if len(emb) != 512 {
		t.Fatalf("expected 512-dim embedding, got %d", len(emb))
	}

	// Проверяем L2-нормализацию
	var sum float64
	for _, v := range emb {
		sum += float64(v) * float64(v)
	}
	norm := math.Sqrt(sum)
	if math.Abs(norm-1.0) > 0.01 {
		t.Fatalf("expected L2 norm ~1.0, got %.4f", norm)
	}
}
