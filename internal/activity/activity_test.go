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
		{"none", counters{1, 2}, counters{1, 2}, types.ActivityNone},
		{"keyboard", counters{1, 2}, counters{2, 2}, types.ActivityKeyboard},
		{"mouse", counters{1, 2}, counters{1, 3}, types.ActivityMouse},
		{"both", counters{1, 2}, counters{2, 3}, types.ActivityBoth},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classify(tt.prev, tt.cur); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
