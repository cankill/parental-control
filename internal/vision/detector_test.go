package vision

import (
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeImageFindsNobodyInBlankImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blank.jpg")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	blank := image.NewRGBA(image.Rect(0, 0, 320, 240))
	if err := jpeg.Encode(file, blank, nil); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	detection, err := AnalyzeImage(path)
	if err != nil {
		t.Fatal(err)
	}
	if detection.Present() || detection.Humans != 0 || detection.Faces != 0 {
		t.Fatalf("blank image detection = %+v", detection)
	}
}

func TestAnalyzeImageRejectsMissingImage(t *testing.T) {
	if _, err := AnalyzeImage(filepath.Join(t.TempDir(), "missing.jpg")); err == nil {
		t.Fatal("missing image was accepted")
	}
}
