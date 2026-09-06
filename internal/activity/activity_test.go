package activity

import (
	"math"
	"testing"
	"time"

	"parental-control/internal/lib/types"
)

func TestCounterDeltaWraparound(t *testing.T) {
	if got := counterDelta(math.MaxUint32-1, 1); got != 3 {
		t.Fatalf("delta = %d, want 3", got)
	}
}

func TestInputSignalStoresOnlyLatestTimestamp(t *testing.T) {
	var signal InputSignal
	if !signal.LastInputAt().IsZero() {
		t.Fatal("new input signal is not empty")
	}
	want := time.Date(2026, 9, 6, 14, 30, 0, 123, time.Local)
	signal.Note(want)
	if got := signal.LastInputAt(); !got.Equal(want) {
		t.Fatalf("last input = %v, want %v", got, want)
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		name      string
		prev, cur counters
		want      types.ActivityKind
	}{
		{"none", counters{keyboard: 1, mouse: 2}, counters{keyboard: 1, mouse: 2}, types.ActivityNone},
		{"keyboard", counters{keyboard: 1, mouse: 2}, counters{keyboard: 2, mouse: 2}, types.ActivityKeyboard},
		{"mouse", counters{keyboard: 1, mouse: 2}, counters{keyboard: 1, mouse: 3}, types.ActivityMouse},
		{"both", counters{keyboard: 1, mouse: 2}, counters{keyboard: 2, mouse: 3}, types.ActivityBoth},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classify(tt.prev, tt.cur); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPresenceInputIgnoresPlainMouseMotion(t *testing.T) {
	previous := counters{keyboard: 10, mouse: 20, presence: 5}
	if hasPresenceInput(previous, counters{keyboard: 10, mouse: 21, presence: 5}) {
		t.Fatal("plain mouse motion was treated as proof of presence")
	}
	if !hasPresenceInput(previous, counters{keyboard: 10, mouse: 21, presence: 6}) {
		t.Fatal("keyboard or mouse action was not treated as proof of presence")
	}
}
