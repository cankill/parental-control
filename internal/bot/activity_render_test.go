package bot

import (
	"bytes"
	"image/color"
	"image/png"
	"math"
	"os"
	"parental-control/internal/lib/types"
	"testing"
)

func TestRenderActivityPNG(t *testing.T) {
	resp := &types.ActivityResponse{TimeStamp: "2026-07-16T09"}
	resp.Buckets[0] = types.ActivityBucket{KeyboardOnlySeconds: 30, MouseOnlySeconds: 60, BothSeconds: 15}
	data, err := renderActivityPNG(resp)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != activityChartWidth || img.Bounds().Dy() != activityChartHeight {
		t.Fatalf("bounds = %v", img.Bounds())
	}
	if bytes.Count(data, []byte("IDAT")) == 0 {
		t.Fatal("not a valid PNG stream")
	}

	barY := activityCenterY - int(activityInnerRadius+20)
	assertPixelColor(t, img.At(activityCenterX, barY), keyboardColor)
	assertPixelColor(t, img.At(activityCenterX+7, barY), mouseColor)
	assertPixelColor(t, img.At(activityCenterX+12, barY), totalColor)

	mouseOnlyY := activityCenterY - int(activityInnerRadius+50)
	assertPixelColor(t, img.At(activityCenterX, mouseOnlyY), mouseColor)
	totalOnlyY := activityCenterY - int(activityInnerRadius+70)
	assertPixelColor(t, img.At(activityCenterX, totalOnlyY), totalColor)
}

func TestRenderEmptyActivityPNG(t *testing.T) {
	if _, err := renderActivityPNG(&types.ActivityResponse{}); err != nil {
		t.Fatal(err)
	}
}

func TestWriteActivityPreview(t *testing.T) {
	path := os.Getenv("ACTIVITY_PREVIEW_PATH")
	if path == "" {
		t.Skip("ACTIVITY_PREVIEW_PATH is not set")
	}
	resp := &types.ActivityResponse{TimeStamp: "2026-07-24T11"}
	resp.Buckets = [12]types.ActivityBucket{
		{KeyboardOnlySeconds: 12, MouseOnlySeconds: 18, BothSeconds: 4},
		{KeyboardOnlySeconds: 45, MouseOnlySeconds: 10, BothSeconds: 8},
		{},
		{KeyboardOnlySeconds: 15, MouseOnlySeconds: 90, BothSeconds: 12},
		{KeyboardOnlySeconds: 120, MouseOnlySeconds: 40, BothSeconds: 25},
		{KeyboardOnlySeconds: 5, MouseOnlySeconds: 5, BothSeconds: 55},
		{KeyboardOnlySeconds: 30, MouseOnlySeconds: 60, BothSeconds: 35},
		{KeyboardOnlySeconds: 80, MouseOnlySeconds: 20, BothSeconds: 10},
		{KeyboardOnlySeconds: 20, MouseOnlySeconds: 105, BothSeconds: 30},
		{KeyboardOnlySeconds: 55, MouseOnlySeconds: 45, BothSeconds: 70},
		{KeyboardOnlySeconds: 8, MouseOnlySeconds: 20, BothSeconds: 6},
		{KeyboardOnlySeconds: 35, MouseOnlySeconds: 70, BothSeconds: 20},
	}
	data, err := renderActivityPNG(resp)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestActivitySeriesForBucket(t *testing.T) {
	series := activitySeriesForBucket(types.ActivityBucket{
		KeyboardOnlySeconds: 30,
		MouseOnlySeconds:    60,
		BothSeconds:         15,
	})
	want := []struct {
		label   string
		seconds int
	}{
		{"Total activity", 105},
		{"Mouse", 75},
		{"Keyboard", 45},
	}
	for i := range want {
		if series[i].label != want[i].label || series[i].seconds != want[i].seconds {
			t.Fatalf("series[%d] = %s/%d, want %s/%d", i, series[i].label, series[i].seconds, want[i].label, want[i].seconds)
		}
	}
}

func TestActivityRadialGeometry(t *testing.T) {
	if got, want := activityBucketAngle(0), -math.Pi/2; math.Abs(got-want) > 1e-9 {
		t.Fatalf("bucket 0 angle = %v, want %v", got, want)
	}
	if got, want := activityBucketAngle(3), 0.0; math.Abs(got-want) > 1e-9 {
		t.Fatalf("bucket 3 angle = %v, want %v", got, want)
	}
	if got, want := activityBarLength(300), activityOuterRadius-activityInnerRadius; math.Abs(got-want) > 1e-9 {
		t.Fatalf("full bar length = %v, want %v", got, want)
	}
	if got := activityBarLength(0); got != 0 {
		t.Fatalf("empty bar length = %v, want 0", got)
	}
	if got, want := activityBarLength(30), (activityOuterRadius-activityInnerRadius)/10; math.Abs(got-want) > 1e-9 {
		t.Fatalf("30-second bar length = %v, want %v", got, want)
	}
}

func assertPixelColor(t *testing.T, got color.Color, want color.RGBA) {
	t.Helper()
	r, g, b, a := got.RGBA()
	gotRGBA := color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
	if gotRGBA != want {
		t.Fatalf("pixel = %#v, want %#v", gotRGBA, want)
	}
}
