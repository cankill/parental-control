package statstorage

import (
	"testing"
	"time"

	"parental-control/internal/lib/types"
)

func TestFinalizedApplicationAggregateIsCachedClonedAndInvalidated(t *testing.T) {
	storage := newTestStorage(t)
	yesterday := time.Now().AddDate(0, 0, -1)
	hour := yesterday.Format(TruncatedToDay) + "T10"
	storage.increaseAppUsageTime(hour, "com.google.Chrome", 60000)

	first := storage.GetStatisticsDay(1)
	if len(first.AppInfos) != 1 || first.AppInfos[0].Duration != time.Minute {
		t.Fatalf("first aggregate = %+v", first.AppInfos)
	}
	beforeHit := storage.CacheMetrics()
	first.AppInfos[0].Duration = 99 * time.Hour
	second := storage.GetStatisticsDay(1)
	afterHit := storage.CacheMetrics()
	if second.AppInfos[0].Duration != time.Minute {
		t.Fatalf("cached response shared mutable data: %+v", second.AppInfos)
	}
	if afterHit.Hits <= beforeHit.Hits {
		t.Fatalf("second request did not hit cache: before=%+v after=%+v", beforeHit, afterHit)
	}

	storage.increaseAppUsageTime(hour, "com.google.Chrome", 30000)
	updated := storage.GetStatisticsDay(1)
	if updated.AppInfos[0].Duration != 90*time.Second {
		t.Fatalf("invalidated aggregate = %+v, want 90s", updated.AppInfos)
	}
}

func TestFinalizedDomainAggregateIsCachedAndInvalidated(t *testing.T) {
	storage := newTestStorage(t)
	yesterday := time.Now().AddDate(0, 0, -1)
	hour := yesterday.Format(TruncatedToDay) + "T11"
	storage.increaseAppUsageTime(domainBucketPrefix+hour, "example.com", 60000)

	_ = storage.GetDomainStatisticsDay(1)
	beforeHit := storage.CacheMetrics()
	cached := storage.GetDomainStatisticsDay(1)
	afterHit := storage.CacheMetrics()
	if len(cached.AppInfos) != 1 || cached.AppInfos[0].Duration != time.Minute || afterHit.Hits <= beforeHit.Hits {
		t.Fatalf("cached domains = %+v metrics=%+v", cached.AppInfos, afterHit)
	}

	storage.increaseAppUsageTime(domainBucketPrefix+hour, "example.com", 15000)
	updated := storage.GetDomainStatisticsDay(1)
	if updated.AppInfos[0].Duration != 75*time.Second {
		t.Fatalf("invalidated domains = %+v, want 75s", updated.AppInfos)
	}
}

func TestActivityCacheInvalidatesBucketsAndPeak(t *testing.T) {
	storage := newTestStorage(t)
	yesterday := time.Now().AddDate(0, 0, -1).Truncate(time.Hour)
	storage.AddActivity([]types.ActivitySample{{At: yesterday, Kind: types.ActivityKeyboard}})

	first := storage.GetActivity(types.ActivityDaily, 1)
	second := storage.GetActivity(types.ActivityDaily, 1)
	if first.Buckets[yesterday.Hour()].ActiveSeconds() != 1 || second.PeakSeconds != 1 {
		t.Fatalf("initial activity: first=%+v peak=%d", first.Buckets[yesterday.Hour()], second.PeakSeconds)
	}
	beforeWrite := storage.CacheMetrics()
	storage.AddActivity([]types.ActivitySample{{At: yesterday, Kind: types.ActivityMouse}})
	updated := storage.GetActivity(types.ActivityDaily, 1)
	if updated.Buckets[yesterday.Hour()].ActiveSeconds() != 2 || updated.PeakSeconds != 2 {
		t.Fatalf("invalidated activity: bucket=%+v peak=%d", updated.Buckets[yesterday.Hour()], updated.PeakSeconds)
	}
	if storage.CacheMetrics().Misses <= beforeWrite.Misses {
		t.Fatal("activity update did not invalidate cached values")
	}
}

func TestCurrentActivityAndPresenceAreCachedUntilNextWrite(t *testing.T) {
	storage := newTestStorage(t)
	now := time.Now().Truncate(time.Minute)
	storage.AddActivity([]types.ActivitySample{{At: now, Kind: types.ActivityKeyboard}})
	storage.AddPresenceSample(types.PresenceSample{At: now, Kind: types.PresencePresent, Seconds: 60})

	first := storage.GetActivity(types.ActivityDaily, 0)
	beforeHit := storage.CacheMetrics()
	second := storage.GetActivity(types.ActivityDaily, 0)
	afterHit := storage.CacheMetrics()
	if first.Buckets[now.Hour()].ActiveSeconds() != 1 || len(first.Presence) != 1 || len(second.Presence) != 1 {
		t.Fatalf("initial current activity = bucket %+v, presence %+v", first.Buckets[now.Hour()], first.Presence)
	}
	if afterHit.Hits < beforeHit.Hits+2 {
		t.Fatalf("current activity did not reuse both caches: before=%+v after=%+v", beforeHit, afterHit)
	}

	storage.AddActivity([]types.ActivitySample{{At: now.Add(time.Second), Kind: types.ActivityMouse}})
	storage.AddPresenceSample(types.PresenceSample{At: now.Add(time.Minute), Kind: types.PresencePresent, Seconds: 60})
	updated := storage.GetActivity(types.ActivityDaily, 0)
	if updated.Buckets[now.Hour()].ActiveSeconds() != 2 {
		t.Fatalf("current activity cache was stale: %+v", updated.Buckets[now.Hour()])
	}
	if len(updated.Presence) != 1 || !updated.Presence[0].End.Equal(now.Add(2*time.Minute)) {
		t.Fatalf("current presence cache was stale: %+v", updated.Presence)
	}
}

func TestActivityPeakTotalsUpdateIncrementally(t *testing.T) {
	storage := newTestStorage(t)
	now := time.Now().Truncate(time.Hour)
	storage.AddActivity([]types.ActivitySample{{At: now, Kind: types.ActivityKeyboard}})
	if got := storage.GetActivity(types.ActivityDaily, 0).PeakSeconds; got != 1 {
		t.Fatalf("initial peak = %d, want 1", got)
	}
	if !storage.index.activityTotalsReady {
		t.Fatal("activity totals index was not initialized")
	}

	storage.AddActivity([]types.ActivitySample{
		{At: now.Add(time.Second), Kind: types.ActivityMouse},
		{At: now.Add(2 * time.Second), Kind: types.ActivityBoth},
	})
	if got := storage.GetActivity(types.ActivityDaily, 0).PeakSeconds; got != 3 {
		t.Fatalf("incremental peak = %d, want 3", got)
	}
	if got := storage.index.activityDayTotals[now.Format(TruncatedToDay)]; got != 3 {
		t.Fatalf("indexed day total = %d, want 3", got)
	}
}

func TestWarmActivityIndexBuildsAllPeakGranularities(t *testing.T) {
	storage := newTestStorage(t)
	now := time.Now().Truncate(time.Hour)
	storage.AddActivity([]types.ActivitySample{{At: now, Kind: types.ActivityBoth}})

	storage.WarmActivityIndex()
	if !storage.index.activityTotalsReady {
		t.Fatal("activity totals index was not warmed")
	}
	for _, period := range []types.ActivityPeriod{types.ActivityHourly, types.ActivityDaily, types.ActivityWeekly} {
		if got := storage.maxActivityPeriodSeconds(period); got != 1 {
			t.Fatalf("warmed peak for period %d = %d, want 1", period, got)
		}
	}
}

func TestPeriodIndexLearnsWritesAfterInitialScan(t *testing.T) {
	storage := newTestStorage(t)
	if _, ok := storage.NearestDayShift(0, true); ok {
		t.Fatal("empty index unexpectedly has a previous day")
	}

	twoDaysAgo := time.Now().AddDate(0, 0, -2).Format(TruncatedToDay) + "T09"
	storage.increaseAppUsageTime(twoDaysAgo, "com.apple.Terminal", 60000)
	if shift, ok := storage.NearestDayShift(0, true); !ok || shift != 2 {
		t.Fatalf("indexed write shift = (%d,%v), want (2,true)", shift, ok)
	}
}

func TestReportCacheIsBounded(t *testing.T) {
	storage := newTestStorage(t)
	cache := storage.ensureCache()
	for i := 0; i < maxReportCacheEntries+10; i++ {
		cache.set(string(rune(i)), i)
	}
	if got := storage.CacheMetrics().Entries; got != maxReportCacheEntries {
		t.Fatalf("cache entries = %d, want %d", got, maxReportCacheEntries)
	}
}
