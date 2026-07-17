package activity

import (
	"math"
	"parental-control/internal/lib/types"
	"testing"
)

func TestCounterDeltaWraparound(t *testing.T) {
	if got := counterDelta(math.MaxUint32-1, 1); got != 3 {
		t.Fatalf("delta = %d, want 3", got)
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
