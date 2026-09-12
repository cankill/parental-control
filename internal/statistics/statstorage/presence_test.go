package statstorage

import (
	"testing"
	"time"

	"parental-control/internal/lib/types"
)

func TestPresenceDailySummaryTracksAbsencesAndAvailability(t *testing.T) {
	storage := newTestStorage(t)
	now := time.Now()
	day := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, time.Local)
	for _, sample := range []types.PresenceSample{
		{At: day, Kind: types.PresenceMissing, Seconds: 60, MissThreshold: 2},
		{At: day.Add(time.Minute), Kind: types.PresenceMissing, Seconds: 60, MissThreshold: 2},
		{At: day.Add(2 * time.Minute), Kind: types.PresencePresent, Seconds: 60},
		{At: day.Add(3 * time.Minute), Kind: types.PresenceUnavailable, Seconds: 60},
		{At: day.Add(10 * time.Minute), Kind: types.PresenceMissing, Seconds: 60, MissThreshold: 1},
	} {
		storage.AddPresenceSample(sample)
	}

	response := storage.GetPresence(types.ActivityDaily, 0)
	if response.PresentSeconds != 60 || response.AbsentSeconds != 180 || response.UnavailableSeconds != 60 {
		t.Fatalf("presence totals = %+v", response)
	}
	if response.AbsenceCount != 2 || response.LongestAbsence != 120 || response.PresencePercent() != 25 {
		t.Fatalf("presence metrics = %+v, percent=%d", response, response.PresencePercent())
	}
	if len(response.Days) != 1 || len(response.Days[0].Absences) != 2 {
		t.Fatalf("presence absences = %+v", response.Days)
	}
	if got := response.Days[0].Absences[0].End; !got.Equal(day.Add(2 * time.Minute)) {
		t.Fatalf("first absence end = %v", got)
	}
}

func TestPresenceNavigationAndFinalizedCacheAreIndependent(t *testing.T) {
	storage := newTestStorage(t)
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, time.Local)
	yesterday := today.AddDate(0, 0, -1)
	storage.AddPresenceSample(types.PresenceSample{At: today, Kind: types.PresencePresent, Seconds: 60})
	storage.AddPresenceSample(types.PresenceSample{At: yesterday, Kind: types.PresenceMissing, Seconds: 60, MissThreshold: 1})

	if shift, ok := storage.NearestPresenceShift(types.ActivityDaily, 0, true); !ok || shift != 1 {
		t.Fatalf("older presence shift = %d, %v", shift, ok)
	}
	if shift, ok := storage.NearestPresenceShift(types.ActivityDaily, 1, false); !ok || shift != 0 {
		t.Fatalf("newer presence shift = %d, %v", shift, ok)
	}

	first := storage.GetPresence(types.ActivityDaily, 1)
	first.Days[0].AbsentSeconds = 999
	second := storage.GetPresence(types.ActivityDaily, 1)
	if second.Days[0].AbsentSeconds != 60 {
		t.Fatalf("cached response shares mutable data: %+v", second.Days[0])
	}
}

func TestPresenceSummaryDebouncesIsolatedCameraMiss(t *testing.T) {
	storage := newTestStorage(t)
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, time.Local)
	storage.AddPresenceSample(types.PresenceSample{
		At: start, Kind: types.PresenceMissing, Seconds: 60, MissThreshold: 3,
	})
	storage.AddPresenceSample(types.PresenceSample{
		At: start.Add(time.Minute), Kind: types.PresencePresent, Seconds: 60, MissThreshold: 3,
	})

	response := storage.GetPresence(types.ActivityDaily, 0)
	if response.AbsentSeconds != 0 || response.AbsenceCount != 0 || response.PresentSeconds != 120 {
		t.Fatalf("isolated miss distorted presence analytics: %+v", response)
	}
}

func TestPresenceIntervalsUseConfirmedStateAndMergeSamples(t *testing.T) {
	storage := newTestStorage(t)
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, time.Local)
	for _, sample := range []types.PresenceSample{
		{At: start, Kind: types.PresencePresent, Seconds: 60, MissThreshold: 3},
		{At: start.Add(time.Minute), Kind: types.PresenceMissing, Seconds: 60, MissThreshold: 3},
		{At: start.Add(2 * time.Minute), Kind: types.PresencePresent, Seconds: 60, MissThreshold: 3},
		{At: start.Add(3 * time.Minute), Kind: types.PresenceMissing, Seconds: 60, MissThreshold: 1},
	} {
		storage.AddPresenceSample(sample)
	}

	intervals := storage.presenceIntervals(start, start.Add(time.Hour))
	if len(intervals) != 1 {
		t.Fatalf("presence intervals = %+v, want one merged interval", intervals)
	}
	if !intervals[0].Start.Equal(start) || !intervals[0].End.Equal(start.Add(3*time.Minute)) {
		t.Fatalf("presence interval = %+v", intervals[0])
	}
}

func TestPresenceWeeklySummaryIncludesDailyBreakdown(t *testing.T) {
	storage := newTestStorage(t)
	weekStart := activityWeekStart(time.Now())
	storage.AddPresenceSample(types.PresenceSample{At: weekStart.Add(9 * time.Hour), Kind: types.PresencePresent, Seconds: 120})
	storage.AddPresenceSample(types.PresenceSample{At: weekStart.AddDate(0, 0, 1).Add(9 * time.Hour), Kind: types.PresenceMissing, Seconds: 60, MissThreshold: 1})

	response := storage.GetPresence(types.ActivityWeekly, 0)
	if len(response.Days) != 7 || response.PresentSeconds != 120 || response.AbsentSeconds != 60 || response.AbsenceCount != 1 {
		t.Fatalf("weekly presence = %+v", response)
	}
}
