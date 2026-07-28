package bot

import (
	"bytes"
	"image/color"
	"image/png"
	"math"
	"os"
	"parental-control/internal/lib/types"
	"path/filepath"
	"testing"
)

func TestRenderActivityPNG(t *testing.T) {
	resp := &types.ActivityResponse{
		Period: types.ActivityHourly, TimeStamp: "2026-07-16T09",
		BucketSeconds: 300, Buckets: make([]types.ActivityBucket, 12),
	}
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

func TestRenderActivityPeriods(t *testing.T) {
	tests := []struct {
		period  types.ActivityPeriod
		buckets int
		seconds int
	}{
		{types.ActivityHourly, 12, 300},
		{types.ActivityDaily, 24, 3600},
		{types.ActivityWeekly, 7, 86400},
	}
	for _, tt := range tests {
		resp := &types.ActivityResponse{
			Period: tt.period, TimeStamp: "period",
			BucketSeconds: tt.seconds, Buckets: make([]types.ActivityBucket, tt.buckets),
		}
		resp.Buckets[0] = types.ActivityBucket{KeyboardOnlySeconds: tt.seconds / 10}
		if _, err := renderActivityPNG(resp); err != nil {
			t.Fatalf("%s: %v", activityPeriodName(tt.period), err)
		}
	}
}

func TestWriteActivityPreview(t *testing.T) {
	path := os.Getenv("ACTIVITY_PREVIEW_PATH")
	if path == "" {
		t.Skip("ACTIVITY_PREVIEW_PATH is not set")
	}
	hourly := &types.ActivityResponse{
		Period: types.ActivityHourly, TimeStamp: "2026-07-28T11",
		BucketSeconds: 300, Buckets: []types.ActivityBucket{
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
		},
	}
	daily := &types.ActivityResponse{
		Period: types.ActivityDaily, TimeStamp: "2026-07-28",
		BucketSeconds: 3600, Buckets: make([]types.ActivityBucket, 24),
	}
	for i := range daily.Buckets {
		daily.Buckets[i] = types.ActivityBucket{
			KeyboardOnlySeconds: (i % 5) * 180,
			MouseOnlySeconds:    (i % 7) * 210,
			BothSeconds:         (i % 3) * 120,
		}
	}
	weekly := &types.ActivityResponse{
		Period: types.ActivityWeekly, TimeStamp: "2026-07-27 – 2026-08-02",
		BucketSeconds: 86400, Buckets: make([]types.ActivityBucket, 7),
	}
	for i := range weekly.Buckets {
		weekly.Buckets[i] = types.ActivityBucket{
			KeyboardOnlySeconds: (i + 1) * 1800,
			MouseOnlySeconds:    (7 - i) * 2100,
			BothSeconds:         (i % 3) * 900,
		}
	}
	ext := filepath.Ext(path)
	base := path[:len(path)-len(ext)]
	for name, resp := range map[string]*types.ActivityResponse{
		"hourly": hourly,
		"daily":  daily,
		"weekly": weekly,
	} {
		data, err := renderActivityPNG(resp)
		if err != nil {
			t.Fatal(err)
		}
		outputPath := base + "-" + name + ext
		if err := os.WriteFile(outputPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
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
	if got, want := activityBucketAngle(0, 12), -math.Pi/2; math.Abs(got-want) > 1e-9 {
		t.Fatalf("bucket 0 angle = %v, want %v", got, want)
	}
	if got, want := activityBucketAngle(3, 12), 0.0; math.Abs(got-want) > 1e-9 {
		t.Fatalf("bucket 3 angle = %v, want %v", got, want)
	}
	if got, want := activityBucketAngle(6, 24), 0.0; math.Abs(got-want) > 1e-9 {
		t.Fatalf("daily bucket 6 angle = %v, want %v", got, want)
	}
	if got, want := activityBarLength(300, 300), activityOuterRadius-activityInnerRadius; math.Abs(got-want) > 1e-9 {
		t.Fatalf("full bar length = %v, want %v", got, want)
	}
	if got := activityBarLength(0, 300); got != 0 {
		t.Fatalf("empty bar length = %v, want 0", got)
	}
	if got, want := activityBarLength(30, 300), (activityOuterRadius-activityInnerRadius)/10; math.Abs(got-want) > 1e-9 {
		t.Fatalf("30-second bar length = %v, want %v", got, want)
	}
}

func TestActivityNavigationData(t *testing.T) {
	data := activityNavigationData(types.ActivityWeekly, 3)
	period, shift, ok := parseActivityTarget(data)
	if !ok || period != types.ActivityWeekly || shift != 3 {
		t.Fatalf("target = (%v,%d,%v)", period, shift, ok)
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
