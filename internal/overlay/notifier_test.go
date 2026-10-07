package overlay

import (
	"math"
	"testing"
	"time"

	"github.com/progrium/darwinkit/macos/foundation"
)

func TestCooldownGate(t *testing.T) {
	start := time.Date(2026, 10, 7, 12, 0, 0, 0, time.Local)
	gate := cooldownGate{cooldown: 20 * time.Second}
	if !gate.allow(start) {
		t.Fatal("first signal was rejected")
	}
	if gate.allow(start.Add(19 * time.Second)) {
		t.Fatal("signal inside cooldown was accepted")
	}
	if !gate.allow(start.Add(20 * time.Second)) {
		t.Fatal("signal at cooldown boundary was rejected")
	}
}

func TestChinPulseHasTwoSmoothPeaks(t *testing.T) {
	for _, progress := range []float64{0.25, 0.75} {
		if got := chinPulseAlpha(progress); math.Abs(got-1) > 1e-9 {
			t.Fatalf("alpha at %.2f = %.3f, want 1", progress, got)
		}
	}
	for _, progress := range []float64{0, 0.5, 1} {
		if got := chinPulseAlpha(progress); math.Abs(got-0.12) > 1e-9 {
			t.Fatalf("alpha at %.2f = %.3f, want 0.12", progress, got)
		}
	}
}

func TestScreenIndexAtPoint(t *testing.T) {
	frames := []foundation.Rect{
		{Origin: foundation.Point{X: 0, Y: 0}, Size: foundation.Size{Width: 1920, Height: 1080}},
		{Origin: foundation.Point{X: 1920, Y: -200}, Size: foundation.Size{Width: 1440, Height: 900}},
	}
	if got := screenIndexAtPoint(foundation.Point{X: 100, Y: 100}, frames); got != 0 {
		t.Fatalf("main screen index = %d", got)
	}
	if got := screenIndexAtPoint(foundation.Point{X: 2000, Y: 0}, frames); got != 1 {
		t.Fatalf("second screen index = %d", got)
	}
	if got := screenIndexAtPoint(foundation.Point{X: -1, Y: 0}, frames); got != -1 {
		t.Fatalf("outside index = %d", got)
	}
}
