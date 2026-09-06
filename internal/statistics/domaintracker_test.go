package statistics

import (
	"parental-control/internal/lib/types"
	"testing"
	"time"
)

func TestMeasuredDomainMillisCapsSleepAndSchedulingGaps(t *testing.T) {
	interval := 3 * time.Second
	for _, test := range []struct {
		raw  int64
		want int64
	}{
		{raw: 2800, want: 2800},
		{raw: 3000, want: 3000},
		{raw: 300000, want: 3000},
		{raw: 0, want: 3000},
	} {
		if got := measuredDomainMillis(test.raw, interval); got != test.want {
			t.Errorf("measuredDomainMillis(%d) = %d, want %d", test.raw, got, test.want)
		}
	}
}

func TestAlignDomainTickDoesNotCreditTimeBeforeBrowserActivation(t *testing.T) {
	now := time.Now()
	tick := types.DomainTick{
		At:              now,
		BrowserBundleID: "com.google.Chrome",
		Domain:          "example.com",
		RawMillis:       3000,
		Millis:          3000,
	}

	got := alignDomainTick(tick, "com.google.Chrome", now.Add(-1200*time.Millisecond))
	if got.Millis < 1199 || got.Millis > 1201 {
		t.Fatalf("aligned duration = %dms, want about 1200ms", got.Millis)
	}
	unchanged := alignDomainTick(tick, "com.apple.loginwindow", now.Add(-time.Second))
	if unchanged.Millis != tick.Millis {
		t.Fatalf("mismatched context changed raw observation: %d", unchanged.Millis)
	}
}
