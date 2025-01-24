package statstorage

import (
	"fmt"
	"parental-control/internal/lib/storage/local/diskvstorage"
	"parental-control/internal/lib/types"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	DbPath          = "./database"
	TruncatedToHour = "2006-01-02T15"
)

type StatsStorage struct {
	localStorage *diskvstorage.LocalStorage
}

var capitalizer = cases.Title(language.English)

func Open() *StatsStorage {
	localStorage := diskvstorage.OpenStorage(DbPath)
	return &StatsStorage{localStorage: localStorage}
}

func (s *StatsStorage) IncreaseStatistics(appName string, fromDate time.Time) time.Time {
	toDate := time.Now()
	hours := toDate.Truncate(time.Hour).Sub(fromDate.Truncate(time.Hour)).Hours() + 1
	for hours > 0 {
		newToDate := fromDate.Truncate(time.Hour).Add(1 * time.Hour)
		newToDate = types.Min(newToDate, toDate)
		s.process(fromDate, newToDate, appName)
		fromDate = newToDate
		hours -= 1
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

func (s *StatsStorage) GetStatisticsShifted(shiftHours time.Duration) *types.AppInfoResponse {
	now := time.Now().Add(-shiftHours * time.Hour)
	bucket := now.Format(TruncatedToHour)
	statistics := s.GetStatistics(bucket)
	return &types.AppInfoResponse{AppInfos: statistics, TimeStamp: bucket}
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
