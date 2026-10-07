package overlay

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/progrium/darwinkit/macos/appkit"
	"github.com/progrium/darwinkit/macos/foundation"
)

const (
	chinSignalDuration = 1200 * time.Millisecond
	chinSignalFrame    = 50 * time.Millisecond
)

type cooldownGate struct {
	mu       sync.Mutex
	cooldown time.Duration
	last     time.Time
}

func (g *cooldownGate) allow(at time.Time) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.last.IsZero() && at.Sub(g.last) < g.cooldown {
		return false
	}
	g.last = at
	return true
}

// Notifier displays a non-activating visual cue on the screen containing the
// pointer. All AppKit mutations are dispatched to the main operation queue.
type Notifier struct {
	gate       cooldownGate
	now        func() time.Time
	generation atomic.Uint64
	mainQueue  foundation.OperationQueue

	initialized bool
	panel       appkit.Panel
	border      appkit.Box
	pill        appkit.Box
	label       appkit.TextField
}

func NewChinNotifier(ctx context.Context, cooldown time.Duration) *Notifier {
	if cooldown < time.Second {
		cooldown = 20 * time.Second
	}
	n := &Notifier{
		gate:      cooldownGate{cooldown: cooldown},
		now:       time.Now,
		mainQueue: foundation.OperationQueue_MainQueue(),
	}
	go func() {
		<-ctx.Done()
		generation := n.generation.Add(1)
		n.onMain(func() { n.hide(generation) })
	}()
	return n
}

func (n *Notifier) Signal() {
	if n == nil || !n.gate.allow(n.now()) {
		return
	}
	generation := n.generation.Add(1)
	n.onMain(func() { n.show(generation) })
	go n.animate(generation)
}

func (n *Notifier) animate(generation uint64) {
	ticker := time.NewTicker(chinSignalFrame)
	defer ticker.Stop()
	frames := int(chinSignalDuration / chinSignalFrame)
	for frame := 1; frame <= frames; frame++ {
		<-ticker.C
		alpha := chinPulseAlpha(float64(frame) / float64(frames))
		n.onMain(func() { n.setAlpha(generation, alpha) })
	}
	n.onMain(func() { n.hide(generation) })
}

func chinPulseAlpha(progress float64) float64 {
	if progress <= 0 || progress >= 1 {
		return 0.12
	}
	// sin² produces two smooth peaks across one 1.2-second signal.
	pulse := math.Sin(2 * math.Pi * progress)
	return 0.12 + 0.88*pulse*pulse
}

func (n *Notifier) onMain(action func()) {
	n.mainQueue.AddOperationWithBlock(action)
}

func (n *Notifier) show(generation uint64) {
	if generation != n.generation.Load() {
		return
	}
	screen, frame := cursorScreen()
	if !n.initialized {
		n.createPanel(screen, frame)
	} else {
		n.layout(frame)
	}
	n.panel.SetAlphaValue(chinPulseAlpha(0))
	n.panel.OrderFrontRegardless()
}

func (n *Notifier) setAlpha(generation uint64, alpha float64) {
	if generation == n.generation.Load() && n.initialized {
		n.panel.SetAlphaValue(alpha)
	}
}

func (n *Notifier) hide(generation uint64) {
	if generation == n.generation.Load() && n.initialized {
		n.panel.SetAlphaValue(0)
		n.panel.OrderOut(n.panel)
	}
}

func (n *Notifier) createPanel(screen appkit.Screen, frame foundation.Rect) {
	style := appkit.WindowStyleMaskBorderless | appkit.WindowStyleMaskNonactivatingPanel
	n.panel = appkit.NewPanelWithContentRectStyleMaskBackingDeferScreen(
		frame, style, appkit.BackingStoreBuffered, false, screen)
	// Keep the autoreleased panel alive while it is hidden between signals.
	n.panel.Retain()
	n.panel.SetOpaque(false)
	n.panel.SetBackgroundColor(appkit.Color_ClearColor())
	n.panel.SetHasShadow(false)
	n.panel.SetIgnoresMouseEvents(true)
	n.panel.SetHidesOnDeactivate(false)
	n.panel.SetReleasedWhenClosed(false)
	n.panel.SetFloatingPanel(true)
	n.panel.SetBecomesKeyOnlyIfNeeded(true)
	n.panel.SetWorksWhenModal(true)
	n.panel.SetLevel(appkit.StatusWindowLevel)
	n.panel.SetCollectionBehavior(
		appkit.WindowCollectionBehaviorCanJoinAllSpaces |
			appkit.WindowCollectionBehaviorFullScreenAuxiliary |
			appkit.WindowCollectionBehaviorStationary |
			appkit.WindowCollectionBehaviorIgnoresCycle)

	content := appkit.NewViewWithFrame(localFrame(frame))
	n.panel.SetContentView(content)
	color := appkit.Color_ColorWithSRGBRedGreenBlueAlpha(1.0, 0.31, 0.08, 0.96)

	n.border = appkit.NewBoxWithFrame(localFrame(frame))
	n.border.SetBoxType(appkit.BoxCustom)
	n.border.SetTitlePosition(appkit.NoTitle)
	n.border.SetFillColor(appkit.Color_ClearColor())
	n.border.SetBorderColor(color)
	n.border.SetBorderWidth(12)
	n.border.SetCornerRadius(18)
	content.AddSubview(n.border)

	n.pill = appkit.NewBox()
	n.pill.SetBoxType(appkit.BoxCustom)
	n.pill.SetTitlePosition(appkit.NoTitle)
	n.pill.SetFillColor(color)
	n.pill.SetBorderWidth(0)
	n.pill.SetCornerRadius(18)
	content.AddSubview(n.pill)

	n.label = appkit.TextField_LabelWithString("Рука у подбородка")
	n.label.SetAlignment(appkit.CenterTextAlignment)
	n.label.SetFont(appkit.Font_BoldSystemFontOfSize(18))
	n.label.SetTextColor(appkit.Color_WhiteColor())
	n.label.SetDrawsBackground(false)
	n.label.SetBordered(false)
	n.label.SetEditable(false)
	n.label.SetSelectable(false)
	content.AddSubview(n.label)

	n.initialized = true
	n.layout(frame)
}

func (n *Notifier) layout(screenFrame foundation.Rect) {
	n.panel.SetFrameDisplay(screenFrame, true)
	local := localFrame(screenFrame)
	inset := 7.0
	n.border.SetFrame(foundation.Rect{
		Origin: foundation.Point{X: inset, Y: inset},
		Size: foundation.Size{
			Width:  math.Max(0, local.Size.Width-2*inset),
			Height: math.Max(0, local.Size.Height-2*inset),
		},
	})
	pillWidth, pillHeight := 250.0, 44.0
	pillFrame := foundation.Rect{
		Origin: foundation.Point{X: (local.Size.Width - pillWidth) / 2, Y: local.Size.Height - pillHeight - 30},
		Size:   foundation.Size{Width: pillWidth, Height: pillHeight},
	}
	n.pill.SetFrame(pillFrame)
	n.label.SetFrame(foundation.Rect{
		Origin: foundation.Point{X: pillFrame.Origin.X + 12, Y: pillFrame.Origin.Y + 10},
		Size:   foundation.Size{Width: pillWidth - 24, Height: 24},
	})
}

func localFrame(screenFrame foundation.Rect) foundation.Rect {
	return foundation.Rect{Size: screenFrame.Size}
}

func cursorScreen() (appkit.Screen, foundation.Rect) {
	screens := appkit.Screen_Screens()
	frames := make([]foundation.Rect, len(screens))
	for i, screen := range screens {
		frames[i] = screen.Frame()
	}
	index := screenIndexAtPoint(appkit.Event_MouseLocation(), frames)
	if index >= 0 {
		return screens[index], frames[index]
	}
	screen := appkit.Screen_MainScreen()
	return screen, screen.Frame()
}

func screenIndexAtPoint(point foundation.Point, frames []foundation.Rect) int {
	for i, frame := range frames {
		if point.X >= frame.Origin.X && point.X < frame.Origin.X+frame.Size.Width &&
			point.Y >= frame.Origin.Y && point.Y < frame.Origin.Y+frame.Size.Height {
			return i
		}
	}
	return -1
}
