package bot

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"parental-control/internal/lib/types"
	"path/filepath"
	"testing"
	"time"
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

	mouseOnlyY := activityCenterY - int(activityInnerRadius+150)
	assertPixelColor(t, img.At(activityCenterX, mouseOnlyY), mouseColor)
	totalOnlyY := activityCenterY - int(activityInnerRadius+250)
	assertPixelColor(t, img.At(activityCenterX, totalOnlyY), totalColor)
}

func TestRenderEmptyActivityPNG(t *testing.T) {
	if _, err := renderActivityPNG(&types.ActivityResponse{}); err != nil {
		t.Fatal(err)
	}
}

func TestRenderActivityPresenceOutline(t *testing.T) {
	start := time.Date(2026, 9, 12, 9, 0, 0, 0, time.Local)
	resp := &types.ActivityResponse{
		Period: types.ActivityHourly, TimeStamp: "2026-09-12T09",
		PeriodStart: start, PeriodEnd: start.Add(time.Hour),
		BucketSeconds: 300, Buckets: make([]types.ActivityBucket, 12),
		Presence: []types.PresenceInterval{{Start: start, End: start.Add(15 * time.Minute)}},
	}
	data, err := renderActivityPNG(resp)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	x, y := activityPolarPoint(activityPresenceRadius, -math.Pi/4)
	assertPixelColorNear(t, img, int(x), int(y), presenceColor)
	x, y = activityPolarPoint(activityPresenceRadius, math.Pi/2)
	assertNoPixelColorNear(t, img, int(x), int(y), presenceColor)
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
	hourStart := time.Date(2026, 7, 28, 11, 0, 0, 0, time.Local)
	hourly := &types.ActivityResponse{
		Period: types.ActivityHourly, TimeStamp: "2026-07-28T11",
		PeriodStart: hourStart, PeriodEnd: hourStart.Add(time.Hour),
		Presence: []types.PresenceInterval{
			{Start: hourStart.Add(2 * time.Minute), End: hourStart.Add(17 * time.Minute)},
			{Start: hourStart.Add(23 * time.Minute), End: hourStart.Add(48 * time.Minute)},
		},
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
	dayStart := time.Date(2026, 7, 28, 0, 0, 0, 0, time.Local)
	daily := &types.ActivityResponse{
		Period: types.ActivityDaily, TimeStamp: "2026-07-28",
		PeriodStart: dayStart, PeriodEnd: dayStart.AddDate(0, 0, 1),
		Presence: []types.PresenceInterval{
			{Start: dayStart.Add(8*time.Hour + 15*time.Minute), End: dayStart.Add(12*time.Hour + 10*time.Minute)},
			{Start: dayStart.Add(13 * time.Hour), End: dayStart.Add(17*time.Hour + 40*time.Minute)},
		},
		BucketSeconds: 3600, Buckets: make([]types.ActivityBucket, 24),
	}
	for i := range daily.Buckets {
		daily.Buckets[i] = types.ActivityBucket{
			KeyboardOnlySeconds: (i % 5) * 180,
			MouseOnlySeconds:    (i % 7) * 210,
			BothSeconds:         (i % 3) * 120,
		}
	}
	weekStart := time.Date(2026, 7, 27, 0, 0, 0, 0, time.Local)
	weekly := &types.ActivityResponse{
		Period: types.ActivityWeekly, TimeStamp: "2026-07-27 – 2026-08-02",
		PeriodStart: weekStart, PeriodEnd: weekStart.AddDate(0, 0, 7),
		BucketSeconds: 86400, Buckets: make([]types.ActivityBucket, 7),
	}
	for day := 0; day < 5; day++ {
		start := weekStart.AddDate(0, 0, day).Add(8*time.Hour + 30*time.Minute)
		weekly.Presence = append(weekly.Presence, types.PresenceInterval{Start: start, End: start.Add(8 * time.Hour)})
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

func TestActivityMetricsAndCaption(t *testing.T) {
	buckets := make([]types.ActivityBucket, 7)
	buckets[0] = types.ActivityBucket{KeyboardOnlySeconds: 10, MouseOnlySeconds: 20}
	buckets[1] = types.ActivityBucket{BothSeconds: 10}
	total, maximum := activityMetrics(buckets)
	if total != 40 || maximum != 30 {
		t.Fatalf("activity metrics = total %d, maximum %d", total, maximum)
	}

	response := &types.ActivityResponse{
		Period:      types.ActivityWeekly,
		TimeStamp:   "2026-08-24 – 2026-08-30",
		Buckets:     make([]types.ActivityBucket, 7),
		PeakSeconds: 2 * (2*60*60 + 28*60 + 40),
	}
	response.Buckets[0] = types.ActivityBucket{BothSeconds: 2*60*60 + 28*60 + 40}
	caption := activityCaption(response)
	if len(caption.Array) != 7 {
		t.Fatalf("caption parts = %#v", caption.Array)
	}
	if caption.Array[0].PlainText != "Week: " || caption.Array[1].RichTextBold.Text.PlainText != "24.08 - 30.08" {
		t.Fatalf("period caption = %#v", caption.Array[:2])
	}
	if caption.Array[2].PlainText != "\nActivity: " || caption.Array[3].RichTextBold.Text.PlainText != "2 hours, 28 minutes, 40 seconds" {
		t.Fatalf("activity caption = %#v", caption.Array[2:4])
	}
	if caption.Array[4].PlainText != "\nEfficiency: " || caption.Array[5].RichTextBold.Text.PlainText != "50%" || caption.Array[6].PlainText != " of record" {
		t.Fatalf("efficiency caption = %#v", caption.Array[4:])
	}
}

func TestNoActivityMessageKeepsPeriodAndNavigation(t *testing.T) {
	response := &types.ActivityResponse{
		Period:     types.ActivityHourly,
		TimeStamp:  "2026-09-05T14",
		HasOlder:   true,
		OlderShift: 3,
	}
	caption := activityCaption(response)
	if len(caption.Array) != 3 || caption.Array[2].PlainText != "\nNo Activity" {
		t.Fatalf("empty caption = %#v", caption.Array)
	}

	message := renderNoActivity(response)
	if len(message.Blocks) != 4 {
		t.Fatalf("empty activity blocks = %d, want period selector, heading, message, and navigation", len(message.Blocks))
	}
	periodButtons := message.Blocks[0].InputRichBlockButtons.Buttons
	if len(periodButtons) != 3 || periodButtons[0].CallbackData != "\factivity-hourly" ||
		periodButtons[1].CallbackData != "\factivity-daily" || periodButtons[2].CallbackData != "\factivity-weekly" {
		t.Fatalf("activity period buttons = %#v", periodButtons)
	}
	if got := message.Blocks[1].InputRichBlockSectionHeading.Text.PlainText; got != "Hour: 05.09 14:00" {
		t.Fatalf("empty activity heading = %q", got)
	}
	if got := message.Blocks[2].InputRichBlockParagraph.Text.PlainText; got != "No Activity" {
		t.Fatalf("empty activity text = %q", got)
	}
	buttons := message.Blocks[3].InputRichBlockButtons.Buttons
	if len(buttons) != 1 || buttons[0].CallbackData != "\factivity-prev|0:3" {
		t.Fatalf("empty activity navigation = %#v", buttons)
	}
}

func TestActivityPhotoKeepsPeriodSelectorAboveChart(t *testing.T) {
	response := &types.ActivityResponse{
		Period: types.ActivityDaily, TimeStamp: "2026-09-05",
		Buckets: make([]types.ActivityBucket, 24), HasOlder: true, OlderShift: 2,
	}
	message := renderActivityPhoto(bytes.NewReader([]byte("png")), response)
	if len(message.Blocks) != 3 {
		t.Fatalf("activity blocks = %d, want period selector, photo, and navigation", len(message.Blocks))
	}
	if message.Blocks[0].InputRichBlockButtons == nil || message.Blocks[1].InputRichBlockPhoto == nil || message.Blocks[2].InputRichBlockButtons == nil {
		t.Fatalf("activity block order = %#v", message.Blocks)
	}
	periodButtons := message.Blocks[0].InputRichBlockButtons.Buttons
	if len(periodButtons) != 3 || periodButtons[2].Text.PlainText != "Week" {
		t.Fatalf("activity period buttons = %#v", periodButtons)
	}
	if buttons := message.Blocks[2].InputRichBlockButtons.Buttons; len(buttons) != 1 || buttons[0].CallbackData != "\factivity-prev|1:2" {
		t.Fatalf("activity navigation = %#v", buttons)
	}
}

func TestActivityScaleFontIsTwentyPercentLarger(t *testing.T) {
	if got, want := activityScaleFontSize, activitySmallFontSize*1.2; math.Abs(got-want) > 1e-9 {
		t.Fatalf("scale font size = %v, want %v", got, want)
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

func assertPixelColorNear(t *testing.T, img image.Image, x, y int, want color.RGBA) {
	t.Helper()
	for dy := -3; dy <= 3; dy++ {
		for dx := -3; dx <= 3; dx++ {
			if color.RGBAModel.Convert(img.At(x+dx, y+dy)).(color.RGBA) == want {
				return
			}
		}
	}
	t.Fatalf("color %#v not found near (%d,%d)", want, x, y)
}

func assertNoPixelColorNear(t *testing.T, img image.Image, x, y int, unwanted color.RGBA) {
	t.Helper()
	for dy := -3; dy <= 3; dy++ {
		for dx := -3; dx <= 3; dx++ {
			if color.RGBAModel.Convert(img.At(x+dx, y+dy)).(color.RGBA) == unwanted {
				t.Fatalf("unexpected color %#v near (%d,%d)", unwanted, x, y)
			}
		}
	}
}
