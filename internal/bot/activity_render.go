package bot

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"parental-control/internal/lib/types"
	"sort"
	"strings"

	"github.com/go-telegram/bot/models"
	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers/rasterizer"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const activityChartWidth, activityChartHeight = 900, 900
const activityCenterX, activityCenterY = 450, 444
const activityInnerRadius, activityOuterRadius = 62.0, 294.0
const activityClockRadius = 320.0

var (
	chartBackground = color.RGBA{248, 250, 252, 255}
	chartAxis       = color.RGBA{30, 41, 59, 255}
	chartMuted      = color.RGBA{100, 116, 139, 255}
	chartGrid       = color.RGBA{226, 232, 240, 255}
	chartGridStrong = color.RGBA{203, 213, 225, 255}
	keyboardColor   = color.RGBA{59, 130, 246, 255}
	mouseColor      = color.RGBA{249, 115, 22, 255}
	totalColor      = color.RGBA{16, 185, 129, 255}
	chartRegular    = mustParseChartFont(goregular.TTF)
	chartBold       = mustParseChartFont(gobold.TTF)
)

type activityChartSeries struct {
	label    string
	seconds  int
	color    color.RGBA
	priority int
}

type activityChartConfig struct {
	period        types.ActivityPeriod
	bucketSeconds int
	periodSeconds int
	gridStep      int
	intervalLabel string
	periodLabel   string
}

func activityConfig(resp *types.ActivityResponse) activityChartConfig {
	switch resp.Period {
	case types.ActivityDaily:
		return activityChartConfig{
			period: types.ActivityDaily, bucketSeconds: 60 * 60,
			periodSeconds: 24 * 60 * 60, gridStep: 15 * 60,
			intervalLabel: "hourly intervals", periodLabel: "day",
		}
	case types.ActivityWeekly:
		return activityChartConfig{
			period: types.ActivityWeekly, bucketSeconds: 24 * 60 * 60,
			periodSeconds: 7 * 24 * 60 * 60, gridStep: 6 * 60 * 60,
			intervalLabel: "daily intervals", periodLabel: "week",
		}
	default:
		return activityChartConfig{
			period: types.ActivityHourly, bucketSeconds: 5 * 60,
			periodSeconds: 60 * 60, gridStep: 60,
			intervalLabel: "5-minute intervals", periodLabel: "hour",
		}
	}
}

func activityPeriodName(period types.ActivityPeriod) string {
	switch period {
	case types.ActivityDaily:
		return "Daily"
	case types.ActivityWeekly:
		return "Weekly"
	default:
		return "Hourly"
	}
}

func activityBuckets(resp *types.ActivityResponse) []types.ActivityBucket {
	if len(resp.Buckets) != 0 {
		return resp.Buckets
	}
	count := 12
	if resp.Period == types.ActivityDaily {
		count = 24
	} else if resp.Period == types.ActivityWeekly {
		count = 7
	}
	return make([]types.ActivityBucket, count)
}

func mustParseChartFont(data []byte) *opentype.Font {
	f, err := opentype.Parse(data)
	if err != nil {
		panic(err)
	}
	return f
}

func newChartFace(f *opentype.Font, size float64) (font.Face, error) {
	return opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
}

func drawText(img draw.Image, face font.Face, x, y int, text string, c color.Color) {
	d := &font.Drawer{Dst: img, Src: image.NewUniform(c), Face: face, Dot: fixed.P(x, y)}
	d.DrawString(text)
}

func drawCenteredText(img draw.Image, face font.Face, center, y int, text string, c color.Color) {
	width := font.MeasureString(face, text).Round()
	drawText(img, face, center-width/2, y, text, c)
}

func activityPolarPoint(radius, angle float64) (float64, float64) {
	return activityCenterX + radius*math.Cos(angle), activityCenterY + radius*math.Sin(angle)
}

func activityBucketAngle(bucket, bucketCount int) float64 {
	return -math.Pi/2 + float64(bucket)*2*math.Pi/float64(bucketCount)
}

func activityBarLength(seconds, bucketSeconds int) float64 {
	if seconds <= 0 {
		return 0
	}
	if seconds > bucketSeconds {
		seconds = bucketSeconds
	}
	length := float64(seconds) * (activityOuterRadius - activityInnerRadius) / float64(bucketSeconds)
	if length < 1 {
		return 1
	}
	return length
}

func drawCanvasLine(ctx *canvas.Context, x0, y0, x1, y1, width float64, c color.Color, rounded bool) {
	ctx.SetFill(nil)
	ctx.SetStrokeColor(c)
	ctx.SetStrokeWidth(width)
	if rounded {
		ctx.SetStrokeCapper(canvas.RoundCap)
	} else {
		ctx.SetStrokeCapper(canvas.ButtCap)
	}
	ctx.DrawPath(x0, y0, canvas.Line(x1-x0, y1-y0))
}

func drawCanvasCircle(ctx *canvas.Context, radius, width float64, fill, stroke color.Color) {
	ctx.SetFill(fill)
	ctx.SetStroke(stroke)
	ctx.SetStrokeWidth(width)
	ctx.DrawPath(activityCenterX, activityCenterY, canvas.Circle(radius))
}

func drawRoundedTopBar(img draw.Image, rect image.Rectangle, radius int, c color.Color) {
	if rect.Empty() {
		return
	}
	if radius > rect.Dx()/2 {
		radius = rect.Dx() / 2
	}
	if radius > rect.Dy() {
		radius = rect.Dy()
	}
	fill := image.NewUniform(c)
	draw.Draw(img, image.Rect(rect.Min.X, rect.Min.Y+radius, rect.Max.X, rect.Max.Y), fill, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(rect.Min.X+radius, rect.Min.Y, rect.Max.X-radius, rect.Min.Y+radius), fill, image.Point{}, draw.Src)
	for y := 0; y < radius; y++ {
		for x := 0; x < radius; x++ {
			dx, dy := radius-x, radius-y
			if dx*dx+dy*dy <= radius*radius {
				img.Set(rect.Min.X+x, rect.Min.Y+y, c)
				img.Set(rect.Max.X-1-x, rect.Min.Y+y, c)
			}
		}
	}
}

func activitySeriesForBucket(bucket types.ActivityBucket) []activityChartSeries {
	series := []activityChartSeries{
		{label: "Total activity", seconds: bucket.ActiveSeconds(), color: totalColor, priority: 0},
		{label: "Keyboard", seconds: bucket.KeyboardOnlySeconds + bucket.BothSeconds, color: keyboardColor, priority: 1},
		{label: "Mouse", seconds: bucket.MouseOnlySeconds + bucket.BothSeconds, color: mouseColor, priority: 2},
	}
	sort.SliceStable(series, func(i, j int) bool {
		if series[i].seconds == series[j].seconds {
			return series[i].priority < series[j].priority
		}
		return series[i].seconds > series[j].seconds
	})
	return series
}

func drawActivityLegendItem(img draw.Image, face font.Face, x, y int, label string, c color.RGBA) {
	drawRoundedTopBar(img, image.Rect(x, y-11, x+14, y+3), 4, c)
	drawText(img, face, x+22, y+1, label, chartAxis)
}

func drawActivityGrid(ctx *canvas.Context, img draw.Image, smallFace font.Face, config activityChartConfig, bucketCount int) {
	for seconds := config.gridStep; seconds <= config.bucketSeconds; seconds += config.gridStep {
		radius := activityInnerRadius + float64(seconds)*(activityOuterRadius-activityInnerRadius)/float64(config.bucketSeconds)
		gridColor := color.Color(chartGrid)
		if seconds == config.bucketSeconds {
			gridColor = chartGridStrong
		}
		drawCanvasCircle(ctx, radius, 1, nil, gridColor)

		labelAngle := -math.Pi / 4
		x, y := activityPolarPoint(radius, labelAngle)
		drawText(img, smallFace, int(x)+5, int(y)+4, formatActivityScale(seconds), chartMuted)
	}

	for bucket := 0; bucket < bucketCount; bucket++ {
		angle := activityBucketAngle(bucket, bucketCount)
		x0, y0 := activityPolarPoint(activityInnerRadius, angle)
		x1, y1 := activityPolarPoint(activityOuterRadius, angle)
		drawCanvasLine(ctx, x0, y0, x1, y1, 1, chartGrid, false)
	}
	drawCanvasCircle(ctx, activityClockRadius, 2, nil, chartAxis)
}

func formatActivityScale(seconds int) string {
	if seconds >= 60*60 && seconds%(60*60) == 0 {
		return fmt.Sprintf("%dh", seconds/(60*60))
	}
	if seconds >= 60 {
		return fmt.Sprintf("%dm", seconds/60)
	}
	return fmt.Sprintf("%ds", seconds)
}

func activityClockLabel(period types.ActivityPeriod, bucket int) (string, bool) {
	switch period {
	case types.ActivityDaily:
		if bucket%6 == 0 {
			return fmt.Sprintf("%02d", bucket), true
		}
	case types.ActivityWeekly:
		labels := [...]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
		if bucket >= 0 && bucket < len(labels) {
			return labels[bucket], true
		}
	default:
		if bucket%3 == 0 {
			return fmt.Sprintf("%02d", bucket*5), true
		}
	}
	return "", false
}

func drawActivityClock(ctx *canvas.Context, img draw.Image, face font.Face, period types.ActivityPeriod, bucketCount int) {
	for bucket := 0; bucket < bucketCount; bucket++ {
		angle := activityBucketAngle(bucket, bucketCount)
		label, major := activityClockLabel(period, bucket)
		tickLength := 8.0
		tickWidth := 1.5
		if major {
			tickLength = 15
			tickWidth = 2.5
		}
		x0, y0 := activityPolarPoint(activityClockRadius-tickLength, angle)
		x1, y1 := activityPolarPoint(activityClockRadius, angle)
		drawCanvasLine(ctx, x0, y0, x1, y1, tickWidth, chartAxis, false)

		if major {
			x, y := activityPolarPoint(activityClockRadius+25, angle)
			width := font.MeasureString(face, label).Round()
			drawText(img, face, int(x)-width/2, int(y)+5, label, chartAxis)
		}
	}
}

func activityBarWidths(bucketCount int) [3]float64 {
	if bucketCount >= 24 {
		return [3]float64{16, 10, 5}
	}
	if bucketCount <= 7 {
		return [3]float64{38, 25, 12}
	}
	return [3]float64{30, 20, 10}
}

func drawActivityBars(ctx *canvas.Context, buckets []types.ActivityBucket, config activityChartConfig) {
	widths := activityBarWidths(len(buckets))
	for bucketIndex, bucket := range buckets {
		angle := activityBucketAngle(bucketIndex, len(buckets))
		x0, y0 := activityPolarPoint(activityInnerRadius, angle)
		for rank, series := range activitySeriesForBucket(bucket) {
			length := activityBarLength(series.seconds, config.bucketSeconds)
			if length == 0 {
				continue
			}
			x1, y1 := activityPolarPoint(activityInnerRadius+length, angle)
			drawCanvasLine(ctx, x0, y0, x1, y1, widths[rank], series.color, true)
		}
	}
}

func renderActivityPNG(resp *types.ActivityResponse) ([]byte, error) {
	regularFace, err := newChartFace(chartRegular, 13)
	if err != nil {
		return nil, fmt.Errorf("create chart font: %w", err)
	}
	defer regularFace.Close()
	smallFace, err := newChartFace(chartRegular, 11)
	if err != nil {
		return nil, fmt.Errorf("create small chart font: %w", err)
	}
	defer smallFace.Close()
	titleFace, err := newChartFace(chartBold, 20)
	if err != nil {
		return nil, fmt.Errorf("create chart title font: %w", err)
	}
	defer titleFace.Close()

	img := image.NewRGBA(image.Rect(0, 0, activityChartWidth, activityChartHeight))
	draw.Draw(img, img.Bounds(), image.NewUniform(chartBackground), image.Point{}, draw.Src)
	ras := rasterizer.FromImage(img, canvas.DPMM(1), canvas.DefaultColorSpace)
	ctx := canvas.NewContext(ras)
	ctx.SetCoordSystem(canvas.CartesianIV)
	config := activityConfig(resp)
	buckets := activityBuckets(resp)

	total := types.ActivityBucket{}
	for _, bucket := range buckets {
		total.KeyboardOnlySeconds += bucket.KeyboardOnlySeconds
		total.MouseOnlySeconds += bucket.MouseOnlySeconds
		total.BothSeconds += bucket.BothSeconds
	}

	drawActivityGrid(ctx, img, smallFace, config, len(buckets))
	drawActivityBars(ctx, buckets, config)
	drawActivityClock(ctx, img, regularFace, config.period, len(buckets))
	drawCanvasCircle(ctx, activityInnerRadius-12, 2, chartBackground, chartAxis)
	ras.Close()

	drawText(img, titleFace, 48, 42, activityPeriodName(config.period)+" input activity", chartAxis)
	drawText(img, smallFace, 48, 65, resp.TimeStamp+" · radial "+config.intervalLabel, chartMuted)
	drawCenteredText(img, titleFace, activityCenterX, activityCenterY-2, formatActivityDuration(total.ActiveSeconds()), chartAxis)
	drawCenteredText(img, smallFace, activityCenterX, activityCenterY+20, "active", chartMuted)

	drawActivityLegendItem(img, regularFace, 48, 852, "Total activity", totalColor)
	drawActivityLegendItem(img, regularFace, 234, 852, "Keyboard", keyboardColor)
	drawActivityLegendItem(img, regularFace, 374, 852, "Mouse", mouseColor)
	summary := fmt.Sprintf("%s active · %.1f%% of %s",
		formatActivityDuration(total.ActiveSeconds()),
		float64(total.ActiveSeconds())*100/float64(config.periodSeconds),
		config.periodLabel,
	)
	drawText(img, regularFace, activityChartWidth-48-font.MeasureString(regularFace, summary).Round(), 853, summary, chartAxis)
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func formatActivityDuration(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	units := []struct {
		seconds          int
		singular, plural string
	}{
		{24 * 60 * 60, "day", "days"},
		{60 * 60, "hour", "hours"},
		{60, "minute", "minutes"},
		{1, "second", "seconds"},
	}
	parts := make([]string, 0, len(units))
	for _, unit := range units {
		value := seconds / unit.seconds
		seconds %= unit.seconds
		if value == 0 {
			continue
		}
		name := unit.plural
		if value == 1 {
			name = unit.singular
		}
		parts = append(parts, fmt.Sprintf("%d %s", value, name))
	}
	if len(parts) == 0 {
		return "0 seconds"
	}
	return strings.Join(parts, ", ")
}

func activityCaption(resp *types.ActivityResponse) string {
	active := 0
	for _, b := range activityBuckets(resp) {
		active += b.ActiveSeconds()
	}
	return fmt.Sprintf("%s activity for %s · %s active",
		activityPeriodName(resp.Period), resp.TimeStamp, formatActivityDuration(active))
}

func activityButtons(resp *types.ActivityResponse) []models.RichMessageButton {
	buttons := []models.RichMessageButton{}
	if resp.HasOlder {
		buttons = append(buttons, richCallbackButton("‹", "activity-prev", activityNavigationData(resp.Period, resp.OlderShift)))
	}
	if resp.HasNewer {
		buttons = append(buttons, richCallbackButton("›", "activity-next", activityNavigationData(resp.Period, resp.NewerShift)))
	}
	return buttons
}

func activityNavigationData(period types.ActivityPeriod, shift int) string {
	return fmt.Sprintf("%d:%d", period, shift)
}
