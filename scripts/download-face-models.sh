#!/bin/bash
# Скачивает ONNX модели для face-кластеризации (YuNet + ArcFace MobileFaceNet).

set -e

MODEL_DIR="${1:-$HOME/.photo-sorter/models}"
mkdir -p "$MODEL_DIR"

echo "Downloading face models to $MODEL_DIR..."

# YuNet face detection (OpenCV Zoo, 232KB)
if [ ! -f "$MODEL_DIR/face-detection.onnx" ]; then
    echo "Downloading YuNet face detection model..."
    curl -L -o "$MODEL_DIR/face-detection.onnx" \
        "https://github.com/opencv/opencv_zoo/raw/main/models/face_detection_yunet/face_detection_yunet_2023mar.onnx"
fi

# ArcFace MobileFaceNet (InsightFace buffalo_s, ~13.6MB)
if [ ! -f "$MODEL_DIR/face-recognition.onnx" ]; then
    echo "Downloading ArcFace MobileFaceNet recognition model..."
    # Скачиваем buffalo_s.zip и извлекаем w600k_mbf.onnx
    TMP_ZIP=$(mktemp)
    curl -L -o "$TMP_ZIP" \
        "https://github.com/deepinsight/insightface/releases/download/v0.7/buffalo_s.zip"
    unzip -j "$TMP_ZIP" "w600k_mbf.onnx" -d "$MODEL_DIR"
    mv "$MODEL_DIR/w600k_mbf.onnx" "$MODEL_DIR/face-recognition.onnx"
    rm -f "$TMP_ZIP"
fi

echo "Done!"
ls -la "$MODEL_DIR"
