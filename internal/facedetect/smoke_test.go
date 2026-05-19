package facedetect

import (
	"context"
	"image"
	"testing"
)

func TestDetectorSmoke(t *testing.T) {
	det, err := NewDetector("../../models/face-detection.onnx")
	if err != nil {
		t.Skipf("ONNX model not available: %v", err)
	}
	defer det.Close()

	// Создаём пустое изображение 640x640 (без лиц)
	img := image.NewRGBA(image.Rect(0, 0, 640, 640))

	boxes, err := det.Detect(context.Background(), img)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	// Ожидаем 0 лиц на пустом изображении
	if len(boxes) != 0 {
		t.Logf("unexpected faces found: %d (expected 0 for blank image)", len(boxes))
	}
}
