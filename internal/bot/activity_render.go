package bot

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"parental-control/internal/lib/types"
	"sort"
	"strconv"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	tele "gopkg.in/telebot.v4"
)

const activityChartWidth, activityChartHeight = 1000, 500
const activityChartLeft, activityChartTop, activityChartBottom, activityChartRight = 70, 78, 405, 970
const activityBucketSeconds = 300

var (
	chartBackground   = color.RGBA{248, 250, 252, 255}
	chartAxis         = color.RGBA{30, 41, 59, 255}
	chartMuted        = color.RGBA{100, 116, 139, 255}
	chartGrid         = color.RGBA{226, 232, 240, 255}
	keyboardColor     = color.RGBA{59, 130, 246, 255}
	mouseColor        = color.RGBA{249, 115, 22, 255}
	totalColor        = color.RGBA{16, 185, 129, 255}
	chartRegular      = mustParseChartFont(goregular.TTF)
	chartBold         = mustParseChartFont(gobold.TTF)
	activityBarWidths = [...]int{46, 30, 16}
)

type activityChartSeries struct {
	label    string
	seconds  int
	color    color.RGBA
	priority int
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

func activityBarHeight(seconds, plotHeight int) int {
	height := (seconds*plotHeight + activityBucketSeconds/2) / activityBucketSeconds
	if seconds > 0 && height == 0 {
		return 1
	}
	return height
}

func drawActivityLegendItem(img draw.Image, face font.Face, x, y int, label string, c color.RGBA) {
	drawRoundedTopBar(img, image.Rect(x, y-11, x+14, y+3), 4, c)
	drawText(img, face, x+22, y+1, label, chartAxis)
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
	plotHeight := activityChartBottom - activityChartTop
	for seconds := 0; seconds <= activityBucketSeconds; seconds += 60 {
		y := activityChartBottom - seconds*plotHeight/activityBucketSeconds
		draw.Draw(img, image.Rect(activityChartLeft, y, activityChartRight, y+1), image.NewUniform(chartGrid), image.Point{}, draw.Src)
		label := "0"
		if seconds != 0 {
			label = fmt.Sprintf("%dm", seconds/60)
		}
		drawText(img, smallFace, 38, y+4, label, chartMuted)
	}
	drawText(img, titleFace, activityChartLeft, 33, "Input activity", chartAxis)
	drawText(img, smallFace, activityChartLeft, 55, resp.TimeStamp+" · active seconds per 5-minute interval", chartMuted)

	total := types.ActivityBucket{}
	for i, bucket := range resp.Buckets {
		center := activityChartLeft + (2*i+1)*(activityChartRight-activityChartLeft)/24
		for rank, series := range activitySeriesForBucket(bucket) {
			height := activityBarHeight(series.seconds, plotHeight)
			if height == 0 {
				continue
			}
			width := activityBarWidths[rank]
			x0 := center - width/2
			drawRoundedTopBar(img, image.Rect(x0, activityChartBottom-height, x0+width, activityChartBottom), 5, series.color)
		}
		if active := bucket.ActiveSeconds(); active > 0 {
			labelY := activityChartBottom - activityBarHeight(active, plotHeight) - 8
			drawCenteredText(img, smallFace, center, labelY, fmt.Sprintf("%ds", active), chartMuted)
		}
		total.KeyboardOnlySeconds += bucket.KeyboardOnlySeconds
		total.MouseOnlySeconds += bucket.MouseOnlySeconds
		total.BothSeconds += bucket.BothSeconds
		drawCenteredText(img, smallFace, center, activityChartBottom+23, fmt.Sprintf("%02d", i*5), chartMuted)
	}
	drawActivityLegendItem(img, regularFace, activityChartLeft, 461, "Total activity", totalColor)
	drawActivityLegendItem(img, regularFace, 255, 461, "Keyboard", keyboardColor)
	drawActivityLegendItem(img, regularFace, 395, 461, "Mouse", mouseColor)
	summary := fmt.Sprintf("%d active sec · %.1f%% of hour", total.ActiveSeconds(), float64(total.ActiveSeconds())/36)
	drawText(img, regularFace, activityChartRight-font.MeasureString(regularFace, summary).Round(), 462, summary, chartAxis)
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func activityCaption(resp *types.ActivityResponse) string {
	active := 0
	for _, b := range resp.Buckets {
		active += b.ActiveSeconds()
	}
	return fmt.Sprintf("Activity for %s · %d active seconds", resp.TimeStamp, active)
}

func activityKeyboard(resp *types.ActivityResponse) *tele.ReplyMarkup {
	kb := &tele.ReplyMarkup{}
	buttons := []tele.Btn{}
	if resp.HasOlder {
		buttons = append(buttons, kb.Data("‹ Earlier", "activity-prev", strconv.Itoa(resp.OlderShift)))
	}
	if resp.HasNewer {
		buttons = append(buttons, kb.Data("Later ›", "activity-next", strconv.Itoa(resp.NewerShift)))
	}
	if len(buttons) != 0 {
		kb.Inline(kb.Row(buttons...))
	}
	return kb
}
