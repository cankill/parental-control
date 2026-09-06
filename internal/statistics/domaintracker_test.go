package statistics

import (
	"bytes"
	"log"
	"parental-control/internal/lib/types"
	"parental-control/internal/statistics/statstorage"
	"strings"
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

func TestLogStatisticsQueryOnlyReportsSlowOperations(t *testing.T) {
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previous) })

	metrics := statstorage.CacheMetrics{Hits: 7, Misses: 3, Entries: 5}
	logStatisticsQuery("daily", 2, time.Now(), metrics)
	if output.Len() != 0 {
		t.Fatalf("fast query was logged: %s", output.String())
	}
	logStatisticsQuery("weekly", 3, time.Now().Add(-slowStatisticsQueryThreshold), metrics)
	if got := output.String(); !strings.Contains(got, "kind=weekly shift=3 duration=") ||
		!strings.Contains(got, "cache_hits=7 cache_misses=3 cache_entries=5") {
		t.Fatalf("slow query log = %q", got)
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
