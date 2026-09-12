package statstorage

import (
	"container/list"
	"strings"
	"time"

	"parental-control/internal/lib/types"
)

const maxReportCacheEntries = 256

type cacheEntry struct {
	key   string
	value any
}

type reportCache struct {
	entries map[string]*list.Element
	recent  *list.List
	hits    uint64
	misses  uint64
}

func newReportCache() *reportCache {
	return &reportCache{entries: make(map[string]*list.Element), recent: list.New()}
}

func (c *reportCache) get(key string) (any, bool) {
	if element, ok := c.entries[key]; ok {
		c.recent.MoveToFront(element)
		c.hits++
		return element.Value.(cacheEntry).value, true
	}
	c.misses++
	return nil, false
}

func (c *reportCache) set(key string, value any) {
	if element, ok := c.entries[key]; ok {
		element.Value = cacheEntry{key: key, value: value}
		c.recent.MoveToFront(element)
		return
	}
	element := c.recent.PushFront(cacheEntry{key: key, value: value})
	c.entries[key] = element
	if c.recent.Len() <= maxReportCacheEntries {
		return
	}
	oldest := c.recent.Back()
	delete(c.entries, oldest.Value.(cacheEntry).key)
	c.recent.Remove(oldest)
}

func (c *reportCache) remove(keys ...string) {
	for _, key := range keys {
		if element, ok := c.entries[key]; ok {
			delete(c.entries, key)
			c.recent.Remove(element)
		}
	}
}

type periodIndex struct {
	appHours               map[string]struct{}
	domainHours            map[string]struct{}
	activityHours          map[string]struct{}
	presenceDays           map[string]struct{}
	availableApps          map[string]struct{}
	availableSites         map[string]struct{}
	availablePresenceHours map[string]struct{}
	activityHourTotals     map[string]int
	activityDayTotals      map[string]int
	activityWeekTotals     map[string]int
	activityPeaks          map[types.ActivityPeriod]int
	appsScanned            bool
	sitesScanned           bool
	presenceHoursScanned   bool
	activityTotalsReady    bool
}

func newPeriodIndex(buckets []string) *periodIndex {
	index := &periodIndex{
		appHours:               make(map[string]struct{}),
		domainHours:            make(map[string]struct{}),
		activityHours:          make(map[string]struct{}),
		presenceDays:           make(map[string]struct{}),
		availableApps:          make(map[string]struct{}),
		availableSites:         make(map[string]struct{}),
		availablePresenceHours: make(map[string]struct{}),
		activityHourTotals:     make(map[string]int),
		activityDayTotals:      make(map[string]int),
		activityWeekTotals:     make(map[string]int),
		activityPeaks:          make(map[types.ActivityPeriod]int),
	}
	for _, bucket := range buckets {
		index.addBucket(bucket)
	}
	return index
}

func (i *periodIndex) addBucket(bucket string) {
	switch {
	case strings.HasPrefix(bucket, contextualDomainBucketPrefix):
		i.addHour(i.domainHours, strings.TrimPrefix(bucket, contextualDomainBucketPrefix))
	case strings.HasPrefix(bucket, domainBucketPrefix):
		i.addHour(i.domainHours, strings.TrimPrefix(bucket, domainBucketPrefix))
	case strings.HasPrefix(bucket, activityBucketPrefix):
		i.addHour(i.activityHours, strings.TrimPrefix(bucket, activityBucketPrefix))
	case strings.HasPrefix(bucket, presenceBucketPrefix):
		i.addDay(i.presenceDays, strings.TrimPrefix(bucket, presenceBucketPrefix))
	default:
		i.addHour(i.appHours, bucket)
	}
}

func (i *periodIndex) addDay(target map[string]struct{}, day string) {
	if _, err := time.ParseInLocation(TruncatedToDay, day, time.Local); err == nil {
		target[day] = struct{}{}
	}
}

func (i *periodIndex) addHour(target map[string]struct{}, hour string) {
	if _, err := time.ParseInLocation(TruncatedToHour, hour, time.Local); err == nil {
		target[hour] = struct{}{}
	}
}

type CacheMetrics struct {
	Hits    uint64
	Misses  uint64
	Entries int
}

func (s *StatsStorage) CacheMetrics() CacheMetrics {
	cache := s.ensureCache()
	return CacheMetrics{Hits: cache.hits, Misses: cache.misses, Entries: cache.recent.Len()}
}

func (s *StatsStorage) ensureCache() *reportCache {
	if s.cache == nil {
		s.cache = newReportCache()
	}
	return s.cache
}

func (s *StatsStorage) ensureIndex() *periodIndex {
	if s.index == nil {
		s.index = newPeriodIndex(s.localStorage.ListBuckets())
	}
	return s.index
}

func (s *StatsStorage) noteBucket(bucket string) {
	if s.index != nil {
		s.index.addBucket(bucket)
	}
}

func (s *StatsStorage) removeCached(keys ...string) {
	if s.cache != nil {
		s.cache.remove(keys...)
	}
}

func (s *StatsStorage) noteUsageWrite(bucket, identity string, milliseconds int64) {
	s.noteBucket(bucket)
	switch {
	case strings.HasPrefix(bucket, domainBucketPrefix):
		hour := strings.TrimPrefix(bucket, domainBucketPrefix)
		s.removeCached(cachePeriodKeys("sites", hour)...)
		if s.index != nil && s.index.sitesScanned && milliseconds > 0 && ShouldTrackDomain(identity) {
			s.index.availableSites[hour] = struct{}{}
		}
	default:
		if _, err := time.ParseInLocation(TruncatedToHour, bucket, time.Local); err != nil {
			return
		}
		s.removeCached(cachePeriodKeys("apps", bucket)...)
		if s.index != nil && s.index.appsScanned && milliseconds > 0 && ShouldTrackApplication(identity) {
			s.index.availableApps[bucket] = struct{}{}
		}
	}
}

func (s *StatsStorage) noteContextualDomainWrite(hour string, usage storedDomainUsage) {
	s.noteBucket(contextualDomainBucketPrefix + hour)
	s.removeCached(cachePeriodKeys("sites", hour)...)
	if s.index != nil && s.index.sitesScanned && usage.Millis > 0 &&
		ShouldTrackApplication(usage.ActiveApplication) &&
		strings.EqualFold(usage.ActiveApplication, usage.BrowserBundleID) &&
		ShouldTrackDomain(usage.Domain) {
		s.index.availableSites[hour] = struct{}{}
	}
}

func (s *StatsStorage) noteActivityWrite(hour string, seconds int) {
	s.noteBucket(activityBucketPrefix + hour)
	s.removeCached(cachePeriodKeys("activity", hour)...)
	if s.index != nil && s.index.activityTotalsReady && seconds > 0 {
		s.index.addActivitySeconds(hour, seconds)
	}
}

func (s *StatsStorage) notePresenceWrite(at time.Time, kind types.PresenceKind) {
	day := at.Format(TruncatedToDay)
	s.noteBucket(presenceBucketPrefix + day)
	s.removeCached("presence:day:"+day, "presence:week:"+activityWeekStart(at).Format(TruncatedToDay))
	s.removeCached(cachePeriodKeys("activity-presence", at.Format(TruncatedToHour))...)
	if s.index != nil && s.index.presenceHoursScanned && kind == types.PresencePresent {
		s.index.availablePresenceHours[at.Format(TruncatedToHour)] = struct{}{}
	}
}

func (s *StatsStorage) appHours() map[string]struct{} {
	return s.ensureIndex().appHours
}

func (s *StatsStorage) activityHours() map[string]struct{} {
	return s.ensureIndex().activityHours
}

func (s *StatsStorage) presenceDays() map[string]struct{} {
	return s.ensureIndex().presenceDays
}

func (s *StatsStorage) availableAppHours() map[string]struct{} {
	index := s.ensureIndex()
	if !index.appsScanned {
		for hour := range index.appHours {
			if len(s.getStatisticsUncached(hour)) > 0 {
				index.availableApps[hour] = struct{}{}
			}
		}
		index.appsScanned = true
	}
	return index.availableApps
}

func (s *StatsStorage) availableDomainHours() map[string]struct{} {
	index := s.ensureIndex()
	if !index.sitesScanned {
		for hour := range index.domainHours {
			if len(s.getDomainStatisticsHourUncached(hour)) > 0 {
				index.availableSites[hour] = struct{}{}
			}
		}
		index.sitesScanned = true
	}
	return index.availableSites
}

func (s *StatsStorage) availablePresenceHours() map[string]struct{} {
	index := s.ensureIndex()
	if !index.presenceHoursScanned {
		for day := range index.presenceDays {
			for _, sample := range resolvePresenceSamples(s.presenceDaySamples(day)) {
				if sample.Kind == types.PresencePresent {
					index.availablePresenceHours[sample.At.Format(TruncatedToHour)] = struct{}{}
				}
			}
		}
		index.presenceHoursScanned = true
	}
	return index.availablePresenceHours
}

func (i *periodIndex) addActivitySeconds(hour string, seconds int) {
	if seconds <= 0 {
		return
	}
	at, err := time.ParseInLocation(TruncatedToHour, hour, time.Local)
	if err != nil {
		return
	}
	day := at.Format(TruncatedToDay)
	week := activityWeekStart(at).Format(TruncatedToDay)
	i.activityHourTotals[hour] += seconds
	i.activityDayTotals[day] += seconds
	i.activityWeekTotals[week] += seconds
	if i.activityHourTotals[hour] > i.activityPeaks[types.ActivityHourly] {
		i.activityPeaks[types.ActivityHourly] = i.activityHourTotals[hour]
	}
	if i.activityDayTotals[day] > i.activityPeaks[types.ActivityDaily] {
		i.activityPeaks[types.ActivityDaily] = i.activityDayTotals[day]
	}
	if i.activityWeekTotals[week] > i.activityPeaks[types.ActivityWeekly] {
		i.activityPeaks[types.ActivityWeekly] = i.activityWeekTotals[week]
	}
}

func cloneAppInfos(source types.AppInfos) types.AppInfos {
	return append(types.AppInfos(nil), source...)
}

func cloneActivityBuckets(source []types.ActivityBucket) []types.ActivityBucket {
	return append([]types.ActivityBucket(nil), source...)
}

func (s *StatsStorage) cachedAppInfos(key string) (types.AppInfos, bool) {
	value, ok := s.ensureCache().get(key)
	if !ok {
		return nil, false
	}
	infos, ok := value.(types.AppInfos)
	return cloneAppInfos(infos), ok
}

func (s *StatsStorage) storeAppInfos(key string, infos types.AppInfos) {
	s.ensureCache().set(key, cloneAppInfos(infos))
}

func (s *StatsStorage) cachedActivityBuckets(key string) ([]types.ActivityBucket, bool) {
	value, ok := s.ensureCache().get(key)
	if !ok {
		return nil, false
	}
	buckets, ok := value.([]types.ActivityBucket)
	return cloneActivityBuckets(buckets), ok
}

func (s *StatsStorage) storeActivityBuckets(key string, buckets []types.ActivityBucket) {
	s.ensureCache().set(key, cloneActivityBuckets(buckets))
}

func clonePresenceIntervals(source []types.PresenceInterval) []types.PresenceInterval {
	return append([]types.PresenceInterval(nil), source...)
}

func (s *StatsStorage) loadPresenceIntervals(key string, load func() []types.PresenceInterval) []types.PresenceInterval {
	if value, ok := s.ensureCache().get(key); ok {
		if intervals, valid := value.([]types.PresenceInterval); valid {
			return clonePresenceIntervals(intervals)
		}
	}
	intervals := load()
	s.ensureCache().set(key, clonePresenceIntervals(intervals))
	return intervals
}

func (s *StatsStorage) loadActivityBuckets(key string, cacheable bool, load func() []types.ActivityBucket) []types.ActivityBucket {
	if cacheable {
		if buckets, ok := s.cachedActivityBuckets(key); ok {
			return buckets
		}
	}
	buckets := load()
	if cacheable {
		s.storeActivityBuckets(key, buckets)
	}
	return buckets
}

func activityCachePeriodName(period types.ActivityPeriod) string {
	switch period {
	case types.ActivityDaily:
		return "daily"
	case types.ActivityWeekly:
		return "weekly"
	default:
		return "hourly"
	}
}

func finalizedHour(hour string) bool {
	start, err := time.ParseInLocation(TruncatedToHour, hour, time.Local)
	return err == nil && time.Now().After(start.Add(time.Hour+time.Minute))
}

func finalizedDay(day string) bool {
	start, err := time.ParseInLocation(TruncatedToDay, day, time.Local)
	if err != nil {
		return false
	}
	today := time.Now()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	return start.Before(today)
}

func finalizedWeek(weekStart time.Time) bool {
	return weekStart.Before(activityWeekStart(time.Now()))
}

func cachePeriodKeys(prefix, hour string) []string {
	at, err := time.ParseInLocation(TruncatedToHour, hour, time.Local)
	if err != nil {
		return []string{prefix + ":hour:" + hour}
	}
	return []string{
		prefix + ":hour:" + hour,
		prefix + ":day:" + at.Format(TruncatedToDay),
		prefix + ":week:" + activityWeekStart(at).Format(TruncatedToDay),
	}
}
