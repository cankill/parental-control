package storage

import (
	"fmt"
	"parental-control/internal/lib/storage/local/nutsdbstorage"
	"parental-control/internal/lib/types"
	"strconv"
	"strings"
	"time"
)

const (
	DbPath     = "./database"
	DateFormat = "2006-01-02T15"
)

type Storage struct {
	localStorage *nutsdbstorage.LocalStorage
}

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

func (s *Storage) IncreaseStatistics(currentBucket string, appName string, currentActivatedAt time.Time) (bucket string, activatedAt time.Time) {
	const op = "storage.IncreaseStatistics"
	activatedAt = time.Now()

	bucket = currentBucket

	hours := activatedAt.Sub(currentActivatedAt).Hours()
	fmt.Printf("%s: Hours between measurements: %f [hours]\n", op, hours)

	periodAppWasActive := activatedAt.UnixMilli() - currentActivatedAt.UnixMilli()

	newBucketName := activatedAt.Format(DateFormat)
	if newBucketName != currentBucket {
		err := s.NewBucket(newBucketName)
		if err != nil {
			fmt.Printf("%s: Lost period: %d [ms] for the app: %s", op, periodAppWasActive, appName)
			return
		}
		bucket = newBucketName
	}

	s.IncreaseStatistic(bucket, appName, periodAppWasActive)

	return
}

func (s *Storage) IncreaseStatistic(bucket string, appName string, periodAppWasActive int64) {
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

func (s *Storage) GetStatistics(bucketName string, appName string) (*int64, error) {
	const op = "storage.GetStatistics"
	storedValue, err := s.localStorage.GetValue(bucketName, appName)
	if err != nil {
		if err.Error() == "key not found" || err.Error() == "bucket not exist" {
			storedValue = []byte("0")
		} else {
			fmt.Printf("%s: Problem getting from bucket (%s) application key (%s) error: %s\n", op, bucketName, appName, err)
			return nil, fmt.Errorf("%s: Can't open local storage at: %s, with the error: %w", op, DbPath, err)
		}
	}

	milliseconds, err := strconv.ParseInt(string(storedValue), 10, 64)
	if err != nil {
		fmt.Printf("Problem converting value: %s to number with error: %s\n", storedValue, err)
		return nil, fmt.Errorf("%s: Can't parse local storage value: %s, with the error: %w", op, storedValue, err)
	}

	return &milliseconds, nil

}

func (s *Storage) GetAppStatistic(appName string) (*int64, error) {
	now := time.Now()
	bucket := now.Format(DateFormat)
	return s.GetStatistics(bucket, appName)
}

func (s *Storage) CalculateStatistics(bucketName string) []types.AppInfo {
	const op = "storage.CalculateStatistics"
	statistics := make([]types.AppInfo, 0)
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

		appName := strings.Title(types.Last(strings.Split(appIdentity, ".")))
		statistics = append(statistics, types.AppInfo{Identity: appName, Time: duration.String()})
	}

	return statistics
}

func (s *Storage) CalculateStatisticsCurrentHour() []types.AppInfo {
	now := time.Now()
	bucket := now.Format(DateFormat)
	return s.CalculateStatistics(bucket)
}
