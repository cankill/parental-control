package statstorage

import (
	"parental-control/internal/lib/storage/local/diskvstorage"
	"parental-control/internal/lib/types"
	"testing"
	"time"
)

func activityStorage(t *testing.T) *StatsStorage {
	t.Helper()
	return &StatsStorage{localStorage: diskvstorage.OpenStorage(t.TempDir())}
}

func TestActivityFiveMinuteAndHourBoundaries(t *testing.T) {
	s := activityStorage(t)
	loc := time.FixedZone("local", 4*60*60)
	s.AddActivity([]types.ActivitySample{
		{At: time.Date(2026, 7, 16, 9, 4, 59, 0, loc), Kind: types.ActivityKeyboard},
		{At: time.Date(2026, 7, 16, 9, 5, 0, 0, loc), Kind: types.ActivityMouse},
		{At: time.Date(2026, 7, 16, 10, 0, 0, 0, loc), Kind: types.ActivityBoth},
	})
	values := s.localStorage.GetValues("activity/2026-07-16T09")
	if values["00"] == "" || values["05"] == "" {
		t.Fatalf("missing five-minute buckets: %#v", values)
	}
	if s.localStorage.GetValue("activity/2026-07-16T10", "00") == "" {
		t.Fatal("missing next-hour bucket")
	}
}

func TestGetActivitySkipsCorruptRecord(t *testing.T) {
	s := activityStorage(t)
	hour := time.Now().Format(TruncatedToHour)
	s.localStorage.SaveValue("activity/"+hour, "00", "not-json")
	if got := s.GetActivity(0).Buckets[0].ActiveSeconds(); got != 0 {
		t.Fatalf("active = %d", got)
	}
}
