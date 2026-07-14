package statstorage

import (
	"fmt"
	"os"
	"parental-control/internal/lib/storage/local/diskvstorage"
	"parental-control/internal/lib/types"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	defaultDbPath   = "./database"
	TruncatedToHour = "2006-01-02T15"
)

type StatsStorage struct {
	localStorage *diskvstorage.LocalStorage
}

var capitalizer = cases.Title(language.English)

// dbPath возвращает каталог БД: из PARENTAL_CONTROL_DB, иначе относительный
// defaultDbPath (зависит от cwd, который в проде задаёт WorkingDirectory в plist).
func dbPath() string {
	if p := os.Getenv("PARENTAL_CONTROL_DB"); p != "" {
		return p
	}
	return defaultDbPath
}

func Open() *StatsStorage {
	localStorage := diskvstorage.OpenStorage(dbPath())
	return &StatsStorage{localStorage: localStorage}
}

func (s *StatsStorage) IncreaseStatistics(appName string, fromDate time.Time) time.Time {
	toDate := time.Now()
	hours := int(toDate.Truncate(time.Hour).Sub(fromDate.Truncate(time.Hour))/time.Hour) + 1
	for hours > 0 {
		newToDate := fromDate.Truncate(time.Hour).Add(1 * time.Hour)
		newToDate = types.Min(newToDate, toDate)
		s.process(fromDate, newToDate, appName)
		fromDate = newToDate
		hours--
	}

	return toDate
}

func (s *StatsStorage) process(fromDate time.Time, toDate time.Time, appName string) {
	periodAppWasActive := toDate.UnixMilli() - fromDate.UnixMilli()
	bucket := fromDate.Format(TruncatedToHour)
	s.increaseAppUsageTime(bucket, appName, periodAppWasActive)
}

func (s *StatsStorage) increaseAppUsageTime(bucket string, appName string, periodAppWasActive int64) {
	const op = "storage.increaseAppUsageTime"
	storedValue := s.localStorage.GetValue(bucket, appName)
	if len(storedValue) == 0 {
		storedValue = "0"
	}

	milliseconds, err := strconv.ParseInt(storedValue, 10, 64)
	if err != nil {
		fmt.Printf("%s: Problem converting value: %s to number with error: %s\n", op, storedValue, err)
		fmt.Printf("%s: Lost period: %d [ms] for the app: %s", op, periodAppWasActive, appName)
		return
	}

	milliseconds += periodAppWasActive
	millisecondsStr := strconv.FormatInt(milliseconds, 10)
	s.localStorage.SaveValue(bucket, appName, millisecondsStr)
}

func (s *StatsStorage) GetStatisticsCurrentHour() types.AppInfos {
	now := time.Now()
	bucket := now.Format(TruncatedToHour)
	return s.GetStatistics(bucket)
}

func (s *StatsStorage) GetStatisticsShifted(shiftHours int) *types.AppInfoResponse {
	now := time.Now().Add(-time.Duration(shiftHours) * time.Hour)
	bucket := now.Format(TruncatedToHour)
	statistics := s.GetStatistics(bucket)
	return &types.AppInfoResponse{AppInfos: statistics, TimeStamp: bucket}
}

// NearestShift находит ближайший shift (часов назад от текущего часа) с реальными
// данными относительно fromShift: older=true — глубже в прошлое (больший shift),
// older=false — ближе к настоящему (меньший shift, не ниже 0). Пропущенные часы
// (дыры в истории) перепрыгиваются. Второе значение — найден ли такой bucket.
func (s *StatsStorage) NearestShift(fromShift int, older bool) (int, bool) {
	currentHour := time.Now().Truncate(time.Hour)
	best := -1
	for _, bucket := range s.localStorage.ListBuckets() {
		t, err := time.ParseInLocation(TruncatedToHour, bucket, time.Local)
		if err != nil {
			continue // не часовой bucket (напр. префиксные домены) — пропускаем
		}
		shift := int(currentHour.Sub(t.Truncate(time.Hour)) / time.Hour)
		if shift < 0 {
			continue
		}
		if older && shift > fromShift {
			if best == -1 || shift < best { // ближайший больший
				best = shift
			}
		}
		if !older && shift < fromShift {
			if shift > best { // ближайший меньший
				best = shift
			}
		}
	}
	return best, best != -1
}

func (s *StatsStorage) DumpBucket(bucketName string) {
	statistics := s.GetStatistics(bucketName)
	fmt.Println(bucketName)
	fmt.Println(statistics.FormatTable())
}

func (s *StatsStorage) GetStatistics(bucketName string) types.AppInfos {
	values := s.localStorage.GetValues(bucketName)
	statistics := mapToAppInfos(values)
	statistics.SortByDurationDesc()
	return statistics
}

// GetStatisticsPeriod агрегирует статистику по всем часовым bucket'ам в диапазоне
// смещений [fromShift, toShift] (в часах назад, включительно), суммируя время
// каждого приложения по всем часам. TimeStamp ответа — человекочитаемый диапазон.
func (s *StatsStorage) GetStatisticsPeriod(fromShift, toShift int) *types.AppInfoResponse {
	if fromShift > toShift {
		fromShift, toShift = toShift, fromShift
	}
	totals := map[string]time.Duration{}
	for shift := fromShift; shift <= toShift; shift++ {
		bucket := time.Now().Add(-time.Duration(shift) * time.Hour).Format(TruncatedToHour)
		for _, ai := range s.GetStatistics(bucket) {
			totals[ai.Identity] += ai.Duration
		}
	}

	stats := make(types.AppInfos, 0, len(totals))
	for name, dur := range totals {
		stats = append(stats, types.AppInfo{Identity: name, Duration: dur})
	}
	stats.SortByDurationDesc()

	from := time.Now().Add(-time.Duration(toShift) * time.Hour).Format(TruncatedToHour)
	to := time.Now().Add(-time.Duration(fromShift) * time.Hour).Format(TruncatedToHour)
	return &types.AppInfoResponse{AppInfos: stats, TimeStamp: from + " … " + to}
}

func (s *StatsStorage) DumpTheUsage() {
	now := time.Now()
	// for range 5 {
	bucket := now.Format(TruncatedToHour)
	s.DumpBucket(bucket)
	// now = now.Add(-1 * time.Hour)
	// }
}

func mapToAppInfos(values map[string]string) types.AppInfos {
	op := "statstorage.mapToAppInfos"
	statistics := types.AppInfos{}
	for appIdentity, millisecondsStr := range values {
		milliseconds, err := strconv.ParseInt(millisecondsStr, 10, 64)
		if err != nil {
			fmt.Printf("%s: Problem converting value: %s to number with error: %s, skipping...\n", op, millisecondsStr, err)
			continue
		}
		duration := time.Duration(milliseconds * 1000000)

		appName := capitalizer.String(types.Last(strings.Split(appIdentity, ".")))
		statistics = append(statistics, types.AppInfo{Identity: appName, Duration: duration})
	}

	return statistics
}
