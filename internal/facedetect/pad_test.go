package facedetect

import (
	"context"
	"image"
	_ "image/jpeg"
	"os"
	"testing"
)

func TestDetectSyntheticFace(t *testing.T) {
	f, err := os.Open("/tmp/synthetic_face.jpg")
	if err != nil {
		t.Skipf("test image not found: %v", err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	det, err := NewDetector("/Users/dmitry/.photo-sorter/models/face-detection.onnx")
	if err != nil {
		t.Skipf("model not available: %v", err)
	}
	defer det.Close()

	boxes, err := det.Detect(context.Background(), img)
	if err != nil {
		t.Fatalf("detect error: %v", err)
	}

	t.Logf("Found %d faces", len(boxes))
	for i, b := range boxes {
		t.Logf("Face %d: score=%.3f box=(%.1f,%.1f,%.1f,%.1f)", i, b.Score, b.X1, b.Y1, b.X2, b.Y2)
	}

	if len(boxes) == 0 {
		t.Fatal("No faces detected on synthetic face image")
	}
	if boxes[0].Score < 0.7 {
		t.Fatalf("Face score too low: %.3f", boxes[0].Score)
	}
}
