package bot

import (
	"bytes"
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
}

func TestRenderEmptyActivityPNG(t *testing.T) {
	if _, err := renderActivityPNG(&types.ActivityResponse{}); err != nil {
		t.Fatal(err)
	}
}
