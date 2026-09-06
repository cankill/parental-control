package statstorage

import (
	"encoding/json"
	"fmt"
	"parental-control/internal/lib/storage/local/diskvstorage"
	"parental-control/internal/lib/types"
	"testing"
	"time"
)

// BenchmarkHistoricalReportsYear models a year of eight-hour workdays. The
// fixture intentionally uses thousands of diskv files so repeated ListBuckets
// scans and aggregate reads remain visible in benchmark results.
func BenchmarkHistoricalReportsYear(b *testing.B) {
	b.StopTimer()
	storage := newYearBenchmarkStorage(b)

	b.Run("ApplicationsDay", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			_ = storage.GetStatisticsDay(180)
		}
	})
	b.Run("ApplicationsWeek", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			_ = storage.GetStatisticsWeek(20)
		}
	})
	b.Run("SitesDay", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			_ = storage.GetDomainStatisticsDay(180)
		}
	})
	b.Run("SitesWeek", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			_ = storage.GetDomainStatisticsWeek(20)
		}
	})
	b.Run("ActivityDay", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			_ = storage.GetActivity(types.ActivityDaily, 180)
		}
	})
	b.Run("ActivityWeek", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			_ = storage.GetActivity(types.ActivityWeekly, 20)
		}
	})
	b.Run("ApplicationsDayNavigation", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			_, _ = storage.NearestDayShift(180, true)
			_, _ = storage.NearestDayShift(180, false)
		}
	})
	b.Run("SitesDayNavigation", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			_, _ = storage.NearestDomainDayShift(180, true)
			_, _ = storage.NearestDomainDayShift(180, false)
		}
	})
}

func newYearBenchmarkStorage(b *testing.B) *StatsStorage {
	b.Helper()
	storage := &StatsStorage{localStorage: diskvstorage.OpenStorage(b.TempDir())}
	start := activityWeekStart(time.Now()).AddDate(0, 0, -364)
	activityJSON, err := json.Marshal(types.ActivityBucket{
		KeyboardOnlySeconds: 60,
		MouseOnlySeconds:    120,
		BothSeconds:         180,
	})
	if err != nil {
		b.Fatal(err)
	}

	for day := 0; day < 365; day++ {
		date := start.AddDate(0, 0, day)
		for hour := 9; hour < 17; hour++ {
			hourName := fmt.Sprintf("%sT%02d", date.Format(TruncatedToDay), hour)
			storage.localStorage.SaveValue(hourName, "com.google.Chrome", "1800000")
			storage.localStorage.SaveValue(hourName, loginWindowBundleID, "1800000")
			storage.localStorage.SaveValue(domainBucketPrefix+hourName, "example.com", "1200000")
			storage.localStorage.SaveValue(domainBucketPrefix+hourName, "newtab", "600000")
			storage.localStorage.SaveValue(activityBucketPrefix+hourName, "00", string(activityJSON))
		}
	}
	return storage
}
