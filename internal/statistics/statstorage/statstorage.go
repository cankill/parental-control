package statstorage

import (
	"crypto/sha256"
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
	defaultDbPath       = "./database"
	loginWindowBundleID = "com.apple.loginwindow"
	TruncatedToHour     = "2006-01-02T15"
	TruncatedToDay      = "2006-01-02"
)

const activityBucketPrefix = "activity/"

type StatsStorage struct {
	localStorage *diskvstorage.LocalStorage
	cache        *reportCache
	index        *periodIndex
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
	s.noteUsageWrite(bucket, appName, milliseconds)
}

const (
	// domainBucketPrefix contains the legacy domain-only counters. Keep reading
	// them so reports retain history written by older versions.
	domainBucketPrefix = "dom/"
	// contextualDomainBucketPrefix stores the observed browser/domain together
	// with the foreground application known by the application event tracker.
	contextualDomainBucketPrefix = "domctx/"
)

type storedDomainUsage struct {
	ActiveApplication string `json:"active_application"`
	BrowserBundleID   string `json:"browser_bundle_id"`
	Domain            string `json:"domain"`
	RawMillis         int64  `json:"raw_millis"`
	Millis            int64  `json:"millis"`
}

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

// AddDomainSample preserves the complete browser observation and its foreground
// application context. Filtering happens only when statistics are calculated.
func (s *StatsStorage) AddDomainSample(activeApplication string, tick types.DomainTick) {
	at := tick.At
	if at.IsZero() {
		at = time.Now()
	}
	rawMillis := tick.RawMillis
	if rawMillis <= 0 {
		rawMillis = tick.Millis
	}
	if rawMillis <= 0 && tick.Millis <= 0 {
		return
	}

	contextKey := activeApplication + "\x00" + tick.BrowserBundleID + "\x00" + tick.Domain
	hash := sha256.Sum256([]byte(contextKey))
	key := fmt.Sprintf("%x", hash[:])
	bucket := contextualDomainBucketPrefix + at.Format(TruncatedToHour)

	usage := storedDomainUsage{
		ActiveApplication: activeApplication,
		BrowserBundleID:   tick.BrowserBundleID,
		Domain:            tick.Domain,
	}
	if stored := s.localStorage.GetValue(bucket, key); stored != "" {
		if err := json.Unmarshal([]byte(stored), &usage); err != nil {
			fmt.Printf("domain: corrupt %s/%s, replacing: %s\n", bucket, key, err)
			usage = storedDomainUsage{
				ActiveApplication: activeApplication,
				BrowserBundleID:   tick.BrowserBundleID,
				Domain:            tick.Domain,
			}
		}
	}
	usage.RawMillis += rawMillis
	usage.Millis += tick.Millis
	data, err := json.Marshal(usage)
	if err != nil {
		return
	}
	s.localStorage.SaveValue(bucket, key, string(data))
	s.noteContextualDomainWrite(at.Format(TruncatedToHour), usage)
}

// GetDomainStatistics возвращает статистику доменов за час shiftHours назад.
func (s *StatsStorage) GetDomainStatistics(shiftHours int) *types.AppInfoResponse {
	hour := time.Now().Add(-time.Duration(shiftHours) * time.Hour).Format(TruncatedToHour)
	stats := s.getDomainStatisticsHour(hour)
	return &types.AppInfoResponse{AppInfos: stats, TimeStamp: hour, ShiftHours: shiftHours}
}

// GetDomainStatisticsDay aggregates browser-domain usage for the same calendar
// day boundaries used by the application statistics report.
func (s *StatsStorage) GetDomainStatisticsDay(dayShift int) *types.AppInfoResponse {
	day := time.Now().AddDate(0, 0, -dayShift).Format(TruncatedToDay)
	cacheKey := "sites:day:" + day
	if finalizedDay(day) {
		if stats, ok := s.cachedAppInfos(cacheKey); ok {
			return &types.AppInfoResponse{AppInfos: stats, TimeStamp: day, ShiftHours: dayShift}
		}
	}
	totals := map[string]time.Duration{}
	for _, hour := range s.domainHours() {
		if !strings.HasPrefix(hour, day+"T") {
			continue
		}
		for _, site := range s.getDomainStatisticsHour(hour) {
			totals[site.Identity] += site.Duration
		}
	}
	response := domainStatisticsResponse(totals, day, dayShift)
	if finalizedDay(day) {
		s.storeAppInfos(cacheKey, response.AppInfos)
	}
	return response
}

// GetDomainStatisticsWeek aggregates browser-domain usage for the same
// Monday-Sunday boundaries used by the application statistics report.
func (s *StatsStorage) GetDomainStatisticsWeek(weekShift int) *types.AppInfoResponse {
	weekStart := activityWeekStart(time.Now()).AddDate(0, 0, -7*weekShift)
	weekEnd := weekStart.AddDate(0, 0, 6)
	cacheKey := "sites:week:" + weekStart.Format(TruncatedToDay)
	if finalizedWeek(weekStart) {
		if stats, ok := s.cachedAppInfos(cacheKey); ok {
			timestamp := weekStart.Format(TruncatedToDay) + " – " + weekEnd.Format(TruncatedToDay)
			return &types.AppInfoResponse{AppInfos: stats, TimeStamp: timestamp, ShiftHours: weekShift}
		}
	}
	totals := map[string]time.Duration{}
	for _, hour := range s.domainHours() {
		t, err := time.ParseInLocation(TruncatedToHour, hour, time.Local)
		if err != nil || t.Before(weekStart) || !t.Before(weekEnd.AddDate(0, 0, 1)) {
			continue
		}
		for _, site := range s.getDomainStatisticsHour(hour) {
			totals[site.Identity] += site.Duration
		}
	}
	timestamp := weekStart.Format(TruncatedToDay) + " – " + weekEnd.Format(TruncatedToDay)
	response := domainStatisticsResponse(totals, timestamp, weekShift)
	if finalizedWeek(weekStart) {
		s.storeAppInfos(cacheKey, response.AppInfos)
	}
	return response
}

func (s *StatsStorage) getDomainStatisticsHour(hour string) types.AppInfos {
	cacheKey := "sites:hour:" + hour
	if finalizedHour(hour) {
		if stats, ok := s.cachedAppInfos(cacheKey); ok {
			return stats
		}
	}
	stats := s.getDomainStatisticsHourUncached(hour)
	if finalizedHour(hour) {
		s.storeAppInfos(cacheKey, stats)
	}
	return stats
}

func (s *StatsStorage) getDomainStatisticsHourUncached(hour string) types.AppInfos {
	totals := map[string]time.Duration{}
	for _, site := range mapDomainsToAppInfos(s.localStorage.GetValues(domainBucketPrefix + hour)) {
		totals[site.Identity] += site.Duration
	}
	for _, site := range mapContextualDomainsToAppInfos(s.localStorage.GetValues(contextualDomainBucketPrefix + hour)) {
		totals[site.Identity] += site.Duration
	}
	stats := make(types.AppInfos, 0, len(totals))
	for domain, duration := range totals {
		stats = append(stats, types.AppInfo{Identity: domain, Duration: duration})
	}
	stats.SortByDurationDesc()
	return stats
}

func mapContextualDomainsToAppInfos(values map[string]string) types.AppInfos {
	totals := map[string]time.Duration{}
	for key, raw := range values {
		var usage storedDomainUsage
		if err := json.Unmarshal([]byte(raw), &usage); err != nil {
			fmt.Printf("domain: corrupt contextual value %s: %s, skipping\n", key, err)
			continue
		}
		if !ShouldTrackApplication(usage.ActiveApplication) ||
			!strings.EqualFold(usage.ActiveApplication, usage.BrowserBundleID) ||
			!ShouldTrackDomain(usage.Domain) || usage.Millis <= 0 {
			continue
		}
		totals[usage.Domain] += time.Duration(usage.Millis) * time.Millisecond
	}
	stats := make(types.AppInfos, 0, len(totals))
	for domain, duration := range totals {
		stats = append(stats, types.AppInfo{Identity: domain, Duration: duration})
	}
	return stats
}

func (s *StatsStorage) domainHours() []string {
	hours := make([]string, 0, len(s.ensureIndex().domainHours))
	for hour := range s.ensureIndex().domainHours {
		hours = append(hours, hour)
	}
	return hours
}

func domainStatisticsResponse(totals map[string]time.Duration, timestamp string, shift int) *types.AppInfoResponse {
	stats := make(types.AppInfos, 0, len(totals))
	for domain, duration := range totals {
		stats = append(stats, types.AppInfo{Identity: domain, Duration: duration})
	}
	stats.SortByDurationDesc()
	return &types.AppInfoResponse{AppInfos: stats, TimeStamp: timestamp, ShiftHours: shift}
}

// mapDomainsToAppInfos как mapToAppInfos, но домен — это уже готовое имя (без
// дробления по точкам, иначе youtube.com превратилось бы в "Com").
func mapDomainsToAppInfos(values map[string]string) types.AppInfos {
	const op = "statstorage.mapDomainsToAppInfos"
	stats := types.AppInfos{}
	for domain, msStr := range values {
		if !ShouldTrackDomain(domain) {
			continue
		}
		ms, err := strconv.ParseInt(msStr, 10, 64)
		if err != nil {
			fmt.Printf("%s: bad value %s: %s, skipping\n", op, msStr, err)
			continue
		}
		if ms <= 0 {
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
	return s.nearestHourShift(fromShift, older)
}

// NearestDomainShift is like NearestShift, but includes both legacy and
// contextual domain buckets.
func (s *StatsStorage) NearestDomainShift(fromShift int, older bool) (int, bool) {
	currentHour := time.Now().Truncate(time.Hour)
	best := -1
	for hour := range s.availableDomainHours() {
		t, _ := time.ParseInLocation(TruncatedToHour, hour, time.Local)
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

func (s *StatsStorage) NearestDomainDayShift(fromShift int, older bool) (int, bool) {
	haveDay := map[string]bool{}
	for hour := range s.availableDomainHours() {
		if _, err := time.ParseInLocation(TruncatedToHour, hour, time.Local); err != nil {
			continue
		}
		haveDay[hour[:len(TruncatedToDay)]] = true
	}
	now := time.Now()
	best := -1
	for shift := 0; shift <= 370; shift++ {
		if !haveDay[now.AddDate(0, 0, -shift).Format(TruncatedToDay)] {
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

func (s *StatsStorage) NearestDomainWeekShift(fromShift int, older bool) (int, bool) {
	currentWeek := activityWeekStart(time.Now())
	seen := map[int]bool{}
	for hour := range s.availableDomainHours() {
		t, err := time.ParseInLocation(TruncatedToHour, hour, time.Local)
		if err != nil {
			continue
		}
		shift := int((activityDayIndex(currentWeek) - activityDayIndex(activityWeekStart(t))) / 7)
		if shift >= 0 {
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
		s.noteActivityWrite(hour)
	}
}

func (s *StatsStorage) GetActivity(period types.ActivityPeriod, shift int) *types.ActivityResponse {
	now := time.Now()
	resp := &types.ActivityResponse{Period: period, Shift: shift}
	switch period {
	case types.ActivityDaily:
		day := now.AddDate(0, 0, -shift)
		resp.TimeStamp = day.Format(TruncatedToDay)
		resp.BucketSeconds = 60 * 60
		resp.Buckets = s.loadActivityBuckets("activity:day:"+resp.TimeStamp, finalizedDay(resp.TimeStamp), func() []types.ActivityBucket {
			buckets := make([]types.ActivityBucket, 24)
			for hour := range buckets {
				hourName := fmt.Sprintf("%sT%02d", resp.TimeStamp, hour)
				buckets[hour] = sumActivityBuckets(s.readActivityHour(hourName))
			}
			return buckets
		})
	case types.ActivityWeekly:
		weekStart := activityWeekStart(now).AddDate(0, 0, -7*shift)
		weekEnd := weekStart.AddDate(0, 0, 6)
		resp.TimeStamp = weekStart.Format(TruncatedToDay) + " – " + weekEnd.Format(TruncatedToDay)
		resp.BucketSeconds = 24 * 60 * 60
		resp.Buckets = s.loadActivityBuckets("activity:week:"+weekStart.Format(TruncatedToDay), finalizedWeek(weekStart), func() []types.ActivityBucket {
			buckets := make([]types.ActivityBucket, 7)
			for day := range buckets {
				date := weekStart.AddDate(0, 0, day).Format(TruncatedToDay)
				for hour := 0; hour < 24; hour++ {
					hourName := fmt.Sprintf("%sT%02d", date, hour)
					addActivityBucket(&buckets[day], sumActivityBuckets(s.readActivityHour(hourName)))
				}
			}
			return buckets
		})
	default:
		resp.Period = types.ActivityHourly
		hour := now.Add(-time.Duration(shift) * time.Hour).Format(TruncatedToHour)
		resp.TimeStamp = hour
		resp.BucketSeconds = 5 * 60
		resp.Buckets = s.loadActivityBuckets("activity:hour:"+hour, finalizedHour(hour), func() []types.ActivityBucket {
			return s.readActivityHourUncached(hour)
		})
	}
	resp.PeakSeconds = s.maxActivityPeriodSeconds(resp.Period)
	return resp
}

// maxActivityPeriodSeconds returns the activity record for the requested
// granularity across all stored periods (hour, day, or Monday-Sunday week).
func (s *StatsStorage) maxActivityPeriodSeconds(period types.ActivityPeriod) int {
	cacheKey := "activity:peak:" + activityCachePeriodName(period)
	if value, ok := s.ensureCache().get(cacheKey); ok {
		if maximum, valid := value.(int); valid {
			return maximum
		}
	}
	totals := map[string]int{}
	for hour := range s.activityHours() {
		t, err := time.ParseInLocation(TruncatedToHour, hour, time.Local)
		if err != nil {
			continue
		}
		key := hour
		switch period {
		case types.ActivityDaily:
			key = t.Format(TruncatedToDay)
		case types.ActivityWeekly:
			key = activityWeekStart(t).Format(TruncatedToDay)
		}
		totals[key] += sumActivityBuckets(s.readActivityHourUncached(hour)).ActiveSeconds()
	}
	maximum := 0
	for _, total := range totals {
		if total > maximum {
			maximum = total
		}
	}
	s.ensureCache().set(cacheKey, maximum)
	return maximum
}

func (s *StatsStorage) readActivityHour(hour string) []types.ActivityBucket {
	return s.loadActivityBuckets("activity:hour:"+hour, finalizedHour(hour), func() []types.ActivityBucket {
		return s.readActivityHourUncached(hour)
	})
}

func (s *StatsStorage) readActivityHourUncached(hour string) []types.ActivityBucket {
	buckets := make([]types.ActivityBucket, 12)
	values := s.localStorage.GetValues(activityBucketPrefix + hour)
	for i := range buckets {
		raw := values[fmt.Sprintf("%02d", i*5)]
		if raw == "" {
			continue
		}
		if err := json.Unmarshal([]byte(raw), &buckets[i]); err != nil {
			fmt.Printf("activity: skipping corrupt bucket %s/%02d: %s\n", hour, i*5, err)
		}
	}
	return buckets
}

func sumActivityBuckets(buckets []types.ActivityBucket) types.ActivityBucket {
	var total types.ActivityBucket
	for _, bucket := range buckets {
		addActivityBucket(&total, bucket)
	}
	return total
}

func addActivityBucket(total *types.ActivityBucket, bucket types.ActivityBucket) {
	total.KeyboardOnlySeconds += bucket.KeyboardOnlySeconds
	total.MouseOnlySeconds += bucket.MouseOnlySeconds
	total.BothSeconds += bucket.BothSeconds
}

func activityWeekStart(t time.Time) time.Time {
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	daysSinceMonday := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -daysSinceMonday)
}

func activityDayIndex(t time.Time) int64 {
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return day.Unix() / int64((24*time.Hour)/time.Second)
}

func (s *StatsStorage) NearestActivityShift(period types.ActivityPeriod, fromShift int, older bool) (int, bool) {
	now := time.Now()
	currentHour := now.Truncate(time.Hour)
	currentWeek := activityWeekStart(now)
	best := -1
	for hour := range s.activityHours() {
		t, err := time.ParseInLocation(TruncatedToHour, hour, time.Local)
		if err != nil {
			continue
		}
		var shift int
		switch period {
		case types.ActivityDaily:
			shift = int(activityDayIndex(now) - activityDayIndex(t))
		case types.ActivityWeekly:
			shift = int((activityDayIndex(currentWeek) - activityDayIndex(activityWeekStart(t))) / 7)
		default:
			shift = int(currentHour.Sub(t.Truncate(time.Hour)) / time.Hour)
		}
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

func (s *StatsStorage) nearestHourShift(fromShift int, older bool) (int, bool) {
	currentHour := time.Now().Truncate(time.Hour)
	best := -1
	for hour := range s.availableAppHours() {
		t, err := time.ParseInLocation(TruncatedToHour, hour, time.Local)
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
	cacheKey := "apps:hour:" + bucketName
	if finalizedHour(bucketName) {
		if statistics, ok := s.cachedAppInfos(cacheKey); ok {
			return statistics
		}
	}
	statistics := s.getStatisticsUncached(bucketName)
	if finalizedHour(bucketName) {
		s.storeAppInfos(cacheKey, statistics)
	}
	return statistics
}

func (s *StatsStorage) getStatisticsUncached(bucketName string) types.AppInfos {
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
	cacheKey := "apps:day:" + day
	if finalizedDay(day) {
		if stats, ok := s.cachedAppInfos(cacheKey); ok {
			return &types.AppInfoResponse{AppInfos: stats, TimeStamp: day, ShiftHours: dayShift}
		}
	}
	totals := map[string]time.Duration{}
	for bucket := range s.appHours() {
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
	if finalizedDay(day) {
		s.storeAppInfos(cacheKey, stats)
	}

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
	for bucket := range s.availableAppHours() {
		haveDay[bucket[:len(TruncatedToDay)]] = true
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

// GetStatisticsWeek aggregates application usage for a Monday–Sunday calendar
// week. weekShift=0 selects the current week, 1 the previous week, and so on.
func (s *StatsStorage) GetStatisticsWeek(weekShift int) *types.AppInfoResponse {
	weekStart := activityWeekStart(time.Now()).AddDate(0, 0, -7*weekShift)
	weekEnd := weekStart.AddDate(0, 0, 6)
	cacheKey := "apps:week:" + weekStart.Format(TruncatedToDay)
	if finalizedWeek(weekStart) {
		if stats, ok := s.cachedAppInfos(cacheKey); ok {
			timestamp := weekStart.Format(TruncatedToDay) + " – " + weekEnd.Format(TruncatedToDay)
			return &types.AppInfoResponse{AppInfos: stats, TimeStamp: timestamp, ShiftHours: weekShift}
		}
	}
	totals := map[string]time.Duration{}
	for bucket := range s.appHours() {
		t, err := time.ParseInLocation(TruncatedToHour, bucket, time.Local)
		if err != nil || t.Before(weekStart) || !t.Before(weekEnd.AddDate(0, 0, 1)) {
			continue
		}
		for _, app := range s.GetStatistics(bucket) {
			totals[app.Identity] += app.Duration
		}
	}

	stats := make(types.AppInfos, 0, len(totals))
	for name, duration := range totals {
		stats = append(stats, types.AppInfo{Identity: name, Duration: duration})
	}
	stats.SortByDurationDesc()
	if finalizedWeek(weekStart) {
		s.storeAppInfos(cacheKey, stats)
	}
	return &types.AppInfoResponse{
		AppInfos:   stats,
		TimeStamp:  weekStart.Format(TruncatedToDay) + " – " + weekEnd.Format(TruncatedToDay),
		ShiftHours: weekShift,
	}
}

// NearestWeekShift finds the closest week with application data, skipping
// empty weeks and buckets that only contain excluded applications.
func (s *StatsStorage) NearestWeekShift(fromShift int, older bool) (int, bool) {
	now := time.Now()
	currentWeek := activityWeekStart(now)
	seen := map[int]bool{}
	for bucket := range s.availableAppHours() {
		t, err := time.ParseInLocation(TruncatedToHour, bucket, time.Local)
		if err != nil {
			continue
		}
		shift := int((activityDayIndex(currentWeek) - activityDayIndex(activityWeekStart(t))) / 7)
		if shift >= 0 {
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
		if !ShouldTrackApplication(appIdentity) {
			continue
		}
		milliseconds, err := strconv.ParseInt(millisecondsStr, 10, 64)
		if err != nil {
			fmt.Printf("%s: Problem converting value: %s to number with error: %s, skipping...\n", op, millisecondsStr, err)
			continue
		}
		if milliseconds <= 0 {
			continue
		}
		duration := time.Duration(milliseconds * 1000000)

		statistics = append(statistics, types.AppInfo{Identity: DisplayName(appIdentity), Duration: duration})
	}

	return statistics
}

// ShouldTrackApplication reports whether stored foreground time for bundleID
// counts in calculated statistics. Raw loginwindow time remains stored so the
// reporting rule can be changed without losing source data.
func ShouldTrackApplication(bundleID string) bool {
	return !strings.EqualFold(bundleID, loginWindowBundleID)
}

// ShouldTrackDomain reports whether a stored browser-domain key represents
// user content. Internal blank/start tabs remain in storage but are excluded
// from reports, totals, and navigation.
func ShouldTrackDomain(domain string) bool {
	normalized := strings.ToLower(strings.TrimSpace(domain))
	normalized = strings.TrimSuffix(normalized, ".")
	switch normalized {
	case "", "blank", "newtab", "new-tab", "new-tab-page", "newtab-page", "startpage", "start-page", "favorites":
		return false
	default:
		return true
	}
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
	if bundleID == "" || !ShouldTrackApplication(bundleID) || s.localStorage.GetValue(appInfoBucket, bundleID) != "" {
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
