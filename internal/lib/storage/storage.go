package storage

import (
	"fmt"
	"parental-control/internal/lib/storage/local/nutsdbstorage"
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

type Storage struct {
	localStorage *nutsdbstorage.LocalStorage
}

var capitalizer = cases.Title(language.English)

func New() (*Storage, error) {
	const op = "storage.New"

	localStorage, err := nutsdbstorage.New(DbPath)
	if err != nil {
		return nil, fmt.Errorf("%s: Can't open local storage at: %s, with the error: %w", op, DbPath, err)
	}

	return &Storage{localStorage: localStorage}, nil
}

func (s *Storage) Close() {
	// const op = "storage.Close"
	s.localStorage.Close()
}

func (s *Storage) NewBucket(bucketName string) error {
	const op = "storage.NewBucket"
	err := s.localStorage.NewBucket(bucketName)
	if err != nil && err.Error() != "bucket is already exist" {
		fmt.Printf("%s: Problem creating new bucket (%s): %s\n", op, bucketName, err)
		return err
	}

	return nil

}

func (s *Storage) IncreaseStatistics(appName string, fromDate time.Time) time.Time {
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

func (s *Storage) process(fromDate time.Time, toDate time.Time, appName string) {
	const op = "storage.process"
	periodAppWasActive := toDate.UnixMilli() - fromDate.UnixMilli()

	bucket := toDate.Format(TruncatedToHour)
	exists, err := s.localStorage.FindBucket(bucket)
	if err != nil {
		fmt.Printf("%s: Lost period: %d [ms] for the app: %s", op, periodAppWasActive, appName)
		return
	}

	if !exists {
		err := s.NewBucket(bucket)
		if err != nil {
			fmt.Printf("%s: Lost period: %d [ms] for the app: %s", op, periodAppWasActive, appName)
		}
	}

	s.increaseAppUsageTime(bucket, appName, periodAppWasActive)
}

func (s *Storage) increaseAppUsageTime(bucket string, appName string, periodAppWasActive int64) {
	const op = "storage.IncreaseStatistic"
	storedValue, err := s.localStorage.GetValue(bucket, appName)
	if err != nil {
		if err.Error() == "key not found" {
			storedValue = []byte("0")
		} else {
			fmt.Printf("%s: Problem getting from bucket (%s) application key (%s) error: %s\n", op, bucket, appName, err)
			fmt.Printf("%s: Lost period: %d [ms] for the app: %s", op, periodAppWasActive, appName)
			return
		}
	}

	milliseconds, err := strconv.ParseInt(string(storedValue), 10, 64)
	if err != nil {
		fmt.Printf("%s: Problem converting value: %s to number with error: %s\n", op, storedValue, err)
		fmt.Printf("%s: Lost period: %d [ms] for the app: %s", op, periodAppWasActive, appName)
		return
	}

	milliseconds += periodAppWasActive
	millisecondsStr := strconv.FormatInt(milliseconds, 10)
	err = s.localStorage.SaveValue(bucket, appName, []byte(millisecondsStr))
	if err != nil {
		fmt.Printf("%s: Problem storing value: %s, for app: %s, to local storage bucket: %s, with error: %s\n", op, millisecondsStr, appName, bucket, err)
		fmt.Printf("%s: Lost period: %d [ms] for the app: %s", op, periodAppWasActive, appName)
		return
	}
}

func (s *Storage) GetStatisticsCurrentHour() types.AppInfos {
	now := time.Now()
	bucket := now.Format(TruncatedToHour)
	return s.CalculateStatistics(bucket)
}

func (s *Storage) CalculateStatistics(bucketName string) types.AppInfos {
	const op = "storage.CalculateStatistics"
	statistics := make(types.AppInfos, 0)
	values, err := s.localStorage.GetValues(bucketName)
	if err != nil {
		fmt.Printf("%s: Problem retreiving all values from the bucket: %s, with error: %s\n", op, bucketName, err)
		return statistics
	}

	for appIdentity, bytes := range values {
		millisecondsStr := string(bytes)
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

func (s *Storage) DumpTheUsage() {
	const op = "storage.DumpTheUsage"
	now := time.Now()
	bucket := now.Format(TruncatedToHour)
	fmt.Printf("%s: Dump the usage for the current hour: %s\n", op, bucket)
	values, err := s.localStorage.GetValues(bucket)
	if err != nil {
		if err.Error() == "bucket not exist" {
			fmt.Printf("%s: No usage statistics yet...\n", op)
		} else {
			fmt.Printf("%s: Problem retreiving all values from the bucket: %s, with error: %s\n", op, bucket, err)
		}
		return
	}

	appInfos := types.AppInfos{}
	for appIdentity, bytes := range values {
		millisecondsStr := string(bytes)
		milliseconds, err := strconv.ParseInt(millisecondsStr, 10, 64)
		if err != nil {
			fmt.Printf("%s: Problem converting value: %s to number with error: %s, skipping...\n", op, millisecondsStr, err)
			continue
		}
		duration := time.Duration(milliseconds * 1000000)

		appName := capitalizer.String(types.Last(strings.Split(appIdentity, ".")))
		appInfos = append(appInfos, types.AppInfo{Identity: appName, Duration: duration})
	}

	appInfos.SortByDurationDesc()
	fmt.Println(appInfos.FormatTable())
}
