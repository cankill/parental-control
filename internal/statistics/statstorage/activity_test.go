package statstorage

import (
	"encoding/json"
	"fmt"
	"parental-control/internal/lib/storage/local/diskvstorage"
	"parental-control/internal/lib/types"
	"testing"
	"time"
)

func activityStorage(t *testing.T) *StatsStorage {
	t.Helper()
	return &StatsStorage{localStorage: diskvstorage.OpenStorage(t.TempDir())}
}

func saveActivityBucket(t *testing.T, s *StatsStorage, at time.Time, bucket types.ActivityBucket) {
	t.Helper()
	data, err := json.Marshal(bucket)
	if err != nil {
		t.Fatal(err)
	}
	minute := at.Minute() / 5 * 5
	s.localStorage.SaveValue(
		activityBucketPrefix+at.Format(TruncatedToHour),
		fmt.Sprintf("%02d", minute),
		string(data),
	)
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
	if got := s.GetActivity(types.ActivityHourly, 0).Buckets[0].ActiveSeconds(); got != 0 {
		t.Fatalf("active = %d", got)
	}
}

func TestGetActivityGroupsHourDayAndWeek(t *testing.T) {
	s := activityStorage(t)
	now := time.Now()
	first := now.Truncate(time.Hour)
	second := first.Add(5 * time.Minute)
	saveActivityBucket(t, s, first, types.ActivityBucket{KeyboardOnlySeconds: 10, BothSeconds: 5})
	saveActivityBucket(t, s, second, types.ActivityBucket{MouseOnlySeconds: 20, BothSeconds: 7})

	hourly := s.GetActivity(types.ActivityHourly, 0)
	if len(hourly.Buckets) != 12 || hourly.BucketSeconds != 300 {
		t.Fatalf("hourly shape = %d/%d", len(hourly.Buckets), hourly.BucketSeconds)
	}
	if hourly.Buckets[0].ActiveSeconds() != 15 || hourly.Buckets[1].ActiveSeconds() != 27 {
		t.Fatalf("hourly buckets = %+v", hourly.Buckets[:2])
	}

	daily := s.GetActivity(types.ActivityDaily, 0)
	if len(daily.Buckets) != 24 || daily.BucketSeconds != 3600 {
		t.Fatalf("daily shape = %d/%d", len(daily.Buckets), daily.BucketSeconds)
	}
	dayBucket := daily.Buckets[now.Hour()]
	if dayBucket.KeyboardOnlySeconds != 10 || dayBucket.MouseOnlySeconds != 20 || dayBucket.BothSeconds != 12 {
		t.Fatalf("daily aggregate = %+v", dayBucket)
	}

	weekly := s.GetActivity(types.ActivityWeekly, 0)
	if len(weekly.Buckets) != 7 || weekly.BucketSeconds != 86400 {
		t.Fatalf("weekly shape = %d/%d", len(weekly.Buckets), weekly.BucketSeconds)
	}
	dayIndex := (int(now.Weekday()) + 6) % 7
	weekBucket := weekly.Buckets[dayIndex]
	if weekBucket.KeyboardOnlySeconds != 10 || weekBucket.MouseOnlySeconds != 20 || weekBucket.BothSeconds != 12 {
		t.Fatalf("weekly aggregate = %+v", weekBucket)
	}
}

func TestGetActivityPeakByPeriod(t *testing.T) {
	now := time.Now().Truncate(time.Hour)
	tests := []struct {
		name   string
		period types.ActivityPeriod
		older  time.Time
	}{
		{"hourly", types.ActivityHourly, now.Add(-2 * time.Hour)},
		{"daily", types.ActivityDaily, now.AddDate(0, 0, -2)},
		{"weekly", types.ActivityWeekly, now.AddDate(0, 0, -14)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := activityStorage(t)
			saveActivityBucket(t, s, now, types.ActivityBucket{BothSeconds: 10})
			saveActivityBucket(t, s, tt.older, types.ActivityBucket{BothSeconds: 40})
			response := s.GetActivity(tt.period, 0)
			if response.PeakSeconds != 40 {
				t.Fatalf("peak seconds = %d, want 40", response.PeakSeconds)
			}
		})
	}
}

func TestNearestActivityShiftByPeriod(t *testing.T) {
	now := time.Now()
	t.Run("hourly", func(t *testing.T) {
		s := activityStorage(t)
		saveActivityBucket(t, s, now, types.ActivityBucket{BothSeconds: 1})
		saveActivityBucket(t, s, now.Add(-3*time.Hour), types.ActivityBucket{BothSeconds: 1})
		if shift, ok := s.NearestActivityShift(types.ActivityHourly, 0, true); !ok || shift != 3 {
			t.Fatalf("older = (%d,%v), want (3,true)", shift, ok)
		}
	})
	t.Run("daily", func(t *testing.T) {
		s := activityStorage(t)
		saveActivityBucket(t, s, now, types.ActivityBucket{BothSeconds: 1})
		saveActivityBucket(t, s, now.AddDate(0, 0, -2), types.ActivityBucket{BothSeconds: 1})
		if shift, ok := s.NearestActivityShift(types.ActivityDaily, 0, true); !ok || shift != 2 {
			t.Fatalf("older = (%d,%v), want (2,true)", shift, ok)
		}
	})
	t.Run("weekly", func(t *testing.T) {
		s := activityStorage(t)
		saveActivityBucket(t, s, now, types.ActivityBucket{BothSeconds: 1})
		saveActivityBucket(t, s, activityWeekStart(now).AddDate(0, 0, -14), types.ActivityBucket{BothSeconds: 1})
		if shift, ok := s.NearestActivityShift(types.ActivityWeekly, 0, true); !ok || shift != 2 {
			t.Fatalf("older = (%d,%v), want (2,true)", shift, ok)
		}
		if shift, ok := s.NearestActivityShift(types.ActivityWeekly, 2, false); !ok || shift != 0 {
			t.Fatalf("newer = (%d,%v), want (0,true)", shift, ok)
		}
	})
}
