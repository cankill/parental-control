package statstorage

import (
	"encoding/json"
	"fmt"
	"os"
	"parental-control/internal/appinfo"
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
	TruncatedToDay  = "2006-01-02"
)

const activityBucketPrefix = "activity/"

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

// domainBucketPrefix отделяет доменную статистику от приложений в общем diskv.
const domainBucketPrefix = "dom/"

// AddDomainTime добавляет ms миллисекунд времени домена в текущий часовой bucket.
// Домены хранятся отдельно от приложений (префикс dom/), поэтому не попадают в
// статистику приложений и в её навигацию.
func (s *StatsStorage) AddDomainTime(domain string, ms int64) {
	if domain == "" || ms <= 0 {
		return
	}
	bucket := domainBucketPrefix + time.Now().Format(TruncatedToHour)
	s.increaseAppUsageTime(bucket, domain, ms)
}

// GetDomainStatistics возвращает статистику доменов за час shiftHours назад.
func (s *StatsStorage) GetDomainStatistics(shiftHours int) *types.AppInfoResponse {
	hour := time.Now().Add(-time.Duration(shiftHours) * time.Hour).Format(TruncatedToHour)
	values := s.localStorage.GetValues(domainBucketPrefix + hour)
	stats := mapDomainsToAppInfos(values)
	stats.SortByDurationDesc()
	return &types.AppInfoResponse{AppInfos: stats, TimeStamp: hour, ShiftHours: shiftHours}
}

// mapDomainsToAppInfos как mapToAppInfos, но домен — это уже готовое имя (без
// дробления по точкам, иначе youtube.com превратилось бы в "Com").
func mapDomainsToAppInfos(values map[string]string) types.AppInfos {
	const op = "statstorage.mapDomainsToAppInfos"
	stats := types.AppInfos{}
	for domain, msStr := range values {
		ms, err := strconv.ParseInt(msStr, 10, 64)
		if err != nil {
			fmt.Printf("%s: bad value %s: %s, skipping\n", op, msStr, err)
			continue
		}
		stats = append(stats, types.AppInfo{Identity: domain, Duration: time.Duration(ms) * time.Millisecond})
	}
	return stats
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
// данными приложений относительно fromShift: older=true — глубже в прошлое,
// older=false — ближе к настоящему (не ниже 0). Пропущенные часы перепрыгиваются.
func (s *StatsStorage) NearestShift(fromShift int, older bool) (int, bool) {
	return s.nearestHourShift("", fromShift, older)
}

// NearestDomainShift — как NearestShift, но по часовым bucket'ам доменов (dom/).
func (s *StatsStorage) NearestDomainShift(fromShift int, older bool) (int, bool) {
	return s.nearestHourShift(domainBucketPrefix, fromShift, older)
}

// AddActivity stores one active second in its local five-minute bucket.
func (s *StatsStorage) AddActivity(samples []types.ActivitySample) {
	for _, sample := range samples {
		minute := sample.At.Minute() / 5 * 5
		hour := sample.At.Format(TruncatedToHour)
		key := fmt.Sprintf("%02d", minute)
		bucketName := activityBucketPrefix + hour
		var bucket types.ActivityBucket
		raw := s.localStorage.GetValue(bucketName, key)
		if raw != "" && json.Unmarshal([]byte(raw), &bucket) != nil {
			fmt.Printf("activity: corrupt %s/%s, replacing\n", bucketName, key)
			bucket = types.ActivityBucket{}
		}
		switch sample.Kind {
		case types.ActivityKeyboard:
			bucket.KeyboardOnlySeconds++
		case types.ActivityMouse:
			bucket.MouseOnlySeconds++
		case types.ActivityBoth:
			bucket.BothSeconds++
		default:
			continue
		}
		data, _ := json.Marshal(bucket)
		s.localStorage.SaveValue(bucketName, key, string(data))
	}
}

func (s *StatsStorage) GetActivity(shiftHours int) *types.ActivityResponse {
	hour := time.Now().Add(-time.Duration(shiftHours) * time.Hour).Format(TruncatedToHour)
	resp := &types.ActivityResponse{TimeStamp: hour, ShiftHours: shiftHours}
	values := s.localStorage.GetValues(activityBucketPrefix + hour)
	for i := range resp.Buckets {
		raw := values[fmt.Sprintf("%02d", i*5)]
		if raw == "" {
			continue
		}
		if err := json.Unmarshal([]byte(raw), &resp.Buckets[i]); err != nil {
			fmt.Printf("activity: skipping corrupt bucket %s/%02d: %s\n", hour, i*5, err)
		}
	}
	return resp
}

func (s *StatsStorage) NearestActivityShift(fromShift int, older bool) (int, bool) {
	return s.nearestHourShift(activityBucketPrefix, fromShift, older)
}

// nearestHourShift обобщает поиск ближайшего непустого часа для bucket'ов с
// заданным префиксом (пустой префикс = статистика приложений, dom/ = домены).
func (s *StatsStorage) nearestHourShift(prefix string, fromShift int, older bool) (int, bool) {
	currentHour := time.Now().Truncate(time.Hour)
	best := -1
	for _, bucket := range s.localStorage.ListBuckets() {
		if !strings.HasPrefix(bucket, prefix) {
			continue
		}
		t, err := time.ParseInLocation(TruncatedToHour, strings.TrimPrefix(bucket, prefix), time.Local)
		if err != nil {
			continue
		}
		shift := int(currentHour.Sub(t.Truncate(time.Hour)) / time.Hour)
		if shift < 0 {
			continue
		}
		if older && shift > fromShift && (best == -1 || shift < best) {
			best = shift
		}
		if !older && shift < fromShift && shift > best {
			best = shift
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

// GetStatisticsDay агрегирует статистику за КАЛЕНДАРНЫЙ день, отстоящий на
// dayShift суток назад (0 = сегодня), суммируя время каждого приложения по всем
// часовым bucket'ам этой даты. TimeStamp ответа — сама дата (YYYY-MM-DD).
// Поля DayShift/OlderShift/NewerShift используются ботом для навигации по дням
// (переиспользуют ShiftHours/*Shift-поля ответа, но в дневном смысле).
func (s *StatsStorage) GetStatisticsDay(dayShift int) *types.AppInfoResponse {
	day := time.Now().AddDate(0, 0, -dayShift).Format(TruncatedToDay)
	totals := map[string]time.Duration{}
	for _, bucket := range s.localStorage.ListBuckets() {
		if !strings.HasPrefix(bucket, day+"T") {
			continue // не относится к этому календарному дню
		}
		for _, ai := range s.GetStatistics(bucket) {
			totals[ai.Identity] += ai.Duration
		}
	}

	stats := make(types.AppInfos, 0, len(totals))
	for name, dur := range totals {
		stats = append(stats, types.AppInfo{Identity: name, Duration: dur})
	}
	stats.SortByDurationDesc()

	return &types.AppInfoResponse{
		AppInfos:   stats,
		TimeStamp:  day,
		ShiftHours: dayShift,
	}
}

// NearestDayShift находит ближайший день с данными относительно fromShift (в сутках
// назад): older=true — дальше в прошлое, older=false — ближе к сегодня. Пустые дни
// перепрыгиваются. Аналог NearestShift, но по календарным дням.
func (s *StatsStorage) NearestDayShift(fromShift int, older bool) (int, bool) {
	// Собираем даты с данными как строки, затем сопоставляем со строкой
	// today - N суток (через AddDate) — устойчиво к таймзонам, в отличие от
	// Truncate(24h), который режет по UTC-полуночи.
	haveDay := map[string]bool{}
	for _, bucket := range s.localStorage.ListBuckets() {
		if i := strings.IndexByte(bucket, 'T'); i >= 0 {
			haveDay[bucket[:i]] = true
		}
	}
	now := time.Now()
	seen := map[int]bool{}
	for shift := 0; shift <= 370; shift++ { // разумная граница истории (год+)
		if haveDay[now.AddDate(0, 0, -shift).Format(TruncatedToDay)] {
			seen[shift] = true
		}
	}
	best := -1
	for shift := range seen {
		if older && shift > fromShift && (best == -1 || shift < best) {
			best = shift
		}
		if !older && shift < fromShift && shift > best {
			best = shift
		}
	}
	return best, best != -1
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

		statistics = append(statistics, types.AppInfo{Identity: DisplayName(appIdentity), Duration: duration})
	}

	return statistics
}

// DisplayName — отображаемое имя приложения из bundle id: последний сегмент после
// точки с заглавной буквы (com.spotify.client → Client). Та же логика используется
// для пометки активного приложения и поиска в /info.
func DisplayName(bundleID string) string {
	return capitalizer.String(types.Last(strings.Split(bundleID, ".")))
}

// appInfoBucket — bucket словаря приложений (bundle id → JSON метаданных).
const appInfoBucket = "apps"

// RememberApp резолвит и сохраняет метаданные приложения в словарь, если bundle id
// ещё не известен. Вызывается при трекинге на смене активного приложения —
// резолв (mdfind) выполняется один раз на приложение, а не на каждом событии.
func (s *StatsStorage) RememberApp(bundleID string) {
	if bundleID == "" || s.localStorage.GetValue(appInfoBucket, bundleID) != "" {
		return
	}
	info := appinfo.Resolve(bundleID)
	data, err := json.Marshal(info)
	if err != nil {
		return
	}
	s.localStorage.SaveValue(appInfoBucket, bundleID, string(data))
}

// FindAppInfoByName ищет в словаре приложения, чьё отображаемое имя совпадает с
// name (без учёта регистра). Может вернуть несколько (разные bundle id с одинаковым
// хвостом), поэтому результат — слайс.
func (s *StatsStorage) FindAppInfoByName(name string) []appinfo.Info {
	var result []appinfo.Info
	values := s.localStorage.GetValues(appInfoBucket)
	for bundleID, raw := range values {
		if !strings.EqualFold(DisplayName(bundleID), name) {
			continue
		}
		var info appinfo.Info
		if err := json.Unmarshal([]byte(raw), &info); err == nil {
			result = append(result, info)
		}
	}
	return result
}
