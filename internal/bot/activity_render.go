package bot

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"parental-control/internal/lib/types"
	"strconv"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	tele "gopkg.in/telebot.v4"
)

const activityChartWidth, activityChartHeight = 1000, 500

var (
	chartBackground = color.RGBA{247, 249, 252, 255}
	chartAxis       = color.RGBA{67, 74, 86, 255}
	keyboardColor   = color.RGBA{70, 125, 230, 255}
	mouseColor      = color.RGBA{243, 157, 61, 255}
	bothColor       = color.RGBA{88, 186, 117, 255}
)

func drawText(img draw.Image, x, y int, text string, c color.Color) {
	d := &font.Drawer{Dst: img, Src: image.NewUniform(c), Face: basicfont.Face7x13, Dot: fixed.P(x, y)}
	d.DrawString(text)
}

func renderActivityPNG(resp *types.ActivityResponse) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, activityChartWidth, activityChartHeight))
	draw.Draw(img, img.Bounds(), image.NewUniform(chartBackground), image.Point{}, draw.Src)
	left, top, bottom, right := 75, 60, 420, 970
	plotHeight := bottom - top
	for pct := 0; pct <= 100; pct += 20 {
		y := bottom - pct*plotHeight/100
		draw.Draw(img, image.Rect(left, y, right, y+1), image.NewUniform(color.RGBA{210, 215, 223, 255}), image.Point{}, draw.Src)
		drawText(img, 35, y+4, fmt.Sprintf("%d%%", pct), chartAxis)
	}
	drawText(img, left, 28, "Keyboard and mouse activity - "+resp.TimeStamp, chartAxis)
	barWidth, gap := 52, 21
	total := types.ActivityBucket{}
	for i, bucket := range resp.Buckets {
		x0 := left + 12 + i*(barWidth+gap)
		y := bottom
		segments := []struct {
			seconds int
			c       color.Color
		}{
			{bucket.KeyboardOnlySeconds, keyboardColor}, {bucket.MouseOnlySeconds, mouseColor}, {bucket.BothSeconds, bothColor},
		}
		for _, segment := range segments {
			height := segment.seconds * plotHeight / 300
			if segment.seconds > 0 && height == 0 {
				height = 1
			}
			draw.Draw(img, image.Rect(x0, y-height, x0+barWidth, y), image.NewUniform(segment.c), image.Point{}, draw.Src)
			y -= height
		}
		total.KeyboardOnlySeconds += bucket.KeyboardOnlySeconds
		total.MouseOnlySeconds += bucket.MouseOnlySeconds
		total.BothSeconds += bucket.BothSeconds
		drawText(img, x0+7, bottom+20, fmt.Sprintf("%02d", i*5), chartAxis)
	}
	drawText(img, left, 458, "Keyboard only", keyboardColor)
	drawText(img, 240, 458, "Mouse only", mouseColor)
	drawText(img, 380, 458, "Both", bothColor)
	drawText(img, 550, 458, fmt.Sprintf("Active: %d sec (%.1f%% of hour)", total.ActiveSeconds(), float64(total.ActiveSeconds())/36), chartAxis)
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
