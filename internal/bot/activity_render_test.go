package bot

import (
	"bytes"
	"image/color"
	"image/png"
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

	center := activityChartLeft + (activityChartRight-activityChartLeft)/24
	assertPixelColor(t, img.At(center, activityChartBottom-10), keyboardColor)
	assertPixelColor(t, img.At(center+10, activityChartBottom-10), mouseColor)
	assertPixelColor(t, img.At(center+20, activityChartBottom-10), totalColor)
	assertPixelColor(t, img.At(center, activityChartBottom-100), totalColor)
}

func TestRenderEmptyActivityPNG(t *testing.T) {
	if _, err := renderActivityPNG(&types.ActivityResponse{}); err != nil {
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

func assertPixelColor(t *testing.T, got color.Color, want color.RGBA) {
	t.Helper()
	r, g, b, a := got.RGBA()
	gotRGBA := color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
	if gotRGBA != want {
		t.Fatalf("pixel = %#v, want %#v", gotRGBA, want)
	}
}
