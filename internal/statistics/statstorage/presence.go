package statstorage

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"parental-control/internal/lib/types"
)

const presenceBucketPrefix = "presence/"

type storedPresenceSample struct {
	At            int64              `json:"at"`
	Kind          types.PresenceKind `json:"kind"`
	Seconds       int                `json:"seconds"`
	MissThreshold int                `json:"miss_threshold"`
}

func (s *StatsStorage) AddPresenceSample(sample types.PresenceSample) {
	if sample.At.IsZero() || sample.Seconds <= 0 || sample.Seconds > 3600 {
		return
	}
	switch sample.Kind {
	case types.PresencePresent, types.PresenceMissing, types.PresenceUnavailable:
	default:
		return
	}
	day := sample.At.Format(TruncatedToDay)
	bucket := presenceBucketPrefix + day
	key := sample.At.Format("15")
	if sample.MissThreshold < 1 {
		sample.MissThreshold = 3
	}
	stored := storedPresenceSample{
		At: sample.At.UnixNano(), Kind: sample.Kind, Seconds: sample.Seconds, MissThreshold: sample.MissThreshold,
	}
	var hour []storedPresenceSample
	if raw := s.localStorage.GetValue(bucket, key); raw != "" {
		if err := json.Unmarshal([]byte(raw), &hour); err != nil {
			fmt.Printf("presence: corrupt hour %s/%s, replacing: %s\n", day, key, err)
			hour = nil
		}
	}
	replaced := false
	for i := range hour {
		if hour[i].At == stored.At {
			hour[i] = stored
			replaced = true
			break
		}
	}
	if !replaced {
		hour = append(hour, stored)
	}
	data, err := json.Marshal(hour)
	if err != nil {
		return
	}
	s.localStorage.SaveValue(bucket, key, string(data))
	s.notePresenceWrite(day)
}

func (s *StatsStorage) GetPresence(period types.ActivityPeriod, shift int) *types.PresenceResponse {
	if shift < 0 {
		shift = 0
	}
	if period != types.ActivityWeekly {
		period = types.ActivityDaily
	}
	now := time.Now()
	if period == types.ActivityWeekly {
		weekStart := activityWeekStart(now).AddDate(0, 0, -7*shift)
		cacheKey := "presence:week:" + weekStart.Format(TruncatedToDay)
		if finalizedWeek(weekStart) {
			if cached, ok := s.cachedPresence(cacheKey); ok {
				cached.Shift = shift
				return cached
			}
		}
		response := &types.PresenceResponse{
			Period: types.ActivityWeekly,
			Shift:  shift,
			TimeStamp: weekStart.Format(TruncatedToDay) + " – " +
				weekStart.AddDate(0, 0, 6).Format(TruncatedToDay),
		}
		for day := 0; day < 7; day++ {
			date := weekStart.AddDate(0, 0, day).Format(TruncatedToDay)
			response.Days = append(response.Days, s.presenceDaySummary(date))
		}
		sumPresenceResponse(response)
		if finalizedWeek(weekStart) {
			s.ensureCache().set(cacheKey, clonePresenceResponse(response))
		}
		return response
	}

	date := now.AddDate(0, 0, -shift).Format(TruncatedToDay)
	cacheKey := "presence:day:" + date
	if finalizedDay(date) {
		if cached, ok := s.cachedPresence(cacheKey); ok {
			cached.Shift = shift
			return cached
		}
	}
	response := &types.PresenceResponse{
		Period:    types.ActivityDaily,
		TimeStamp: date,
		Shift:     shift,
		Days:      []types.PresenceDaySummary{s.presenceDaySummary(date)},
	}
	sumPresenceResponse(response)
	if finalizedDay(date) {
		s.ensureCache().set(cacheKey, clonePresenceResponse(response))
	}
	return response
}

func (s *StatsStorage) cachedPresence(key string) (*types.PresenceResponse, bool) {
	value, ok := s.ensureCache().get(key)
	if !ok {
		return nil, false
	}
	response, ok := value.(*types.PresenceResponse)
	if !ok {
		return nil, false
	}
	return clonePresenceResponse(response), true
}

func clonePresenceResponse(source *types.PresenceResponse) *types.PresenceResponse {
	if source == nil {
		return nil
	}
	clone := *source
	clone.Days = append([]types.PresenceDaySummary(nil), source.Days...)
	for i := range clone.Days {
		clone.Days[i].Absences = append([]types.PresenceAbsence(nil), source.Days[i].Absences...)
	}
	return &clone
}

func (s *StatsStorage) presenceDaySummary(day string) types.PresenceDaySummary {
	values := s.localStorage.GetValues(presenceBucketPrefix + day)
	samples := make([]types.PresenceSample, 0, len(values))
	for key, raw := range values {
		var hour []storedPresenceSample
		if err := json.Unmarshal([]byte(raw), &hour); err != nil {
			fmt.Printf("presence: skipping corrupt sample %s/%s: %s\n", day, key, err)
			continue
		}
		for _, stored := range hour {
			at := time.Unix(0, stored.At)
			if stored.At <= 0 || at.Format(TruncatedToDay) != day || stored.Seconds <= 0 || stored.Seconds > 3600 {
				continue
			}
			threshold := stored.MissThreshold
			if threshold < 1 {
				threshold = 3
			}
			samples = append(samples, types.PresenceSample{
				At: at, Kind: stored.Kind, Seconds: stored.Seconds, MissThreshold: threshold,
			})
		}
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i].At.Before(samples[j].At) })

	summary := types.PresenceDaySummary{Date: day}
	var current *types.PresenceAbsence
	missingSamples := 0
	missingThreshold := 0
	var previous types.PresenceSample
	finishAbsence := func() {
		if current == nil {
			return
		}
		if missingSamples >= missingThreshold {
			summary.AbsentSeconds += current.Seconds
			summary.Absences = append(summary.Absences, *current)
			summary.AbsenceCount++
			if current.Seconds > summary.LongestAbsence {
				summary.LongestAbsence = current.Seconds
			}
		} else {
			// The live monitor applies the same debounce before declaring an
			// absence. Short isolated misses are treated as presence so camera
			// noise does not distort the historical report.
			summary.PresentSeconds += current.Seconds
		}
		current = nil
		missingSamples = 0
		missingThreshold = 0
	}
	for _, sample := range samples {
		switch sample.Kind {
		case types.PresencePresent:
			finishAbsence()
			summary.PresentSeconds += sample.Seconds
		case types.PresenceUnavailable:
			finishAbsence()
			summary.UnavailableSeconds += sample.Seconds
		case types.PresenceMissing:
			continuous := current != nil && sample.At.Sub(previous.At) <=
				time.Duration(maxInt(sample.Seconds, previous.Seconds)+5)*time.Second
			if !continuous {
				finishAbsence()
				current = &types.PresenceAbsence{Start: sample.At}
				missingThreshold = sample.MissThreshold
			}
			current.End = sample.At.Add(time.Duration(sample.Seconds) * time.Second)
			current.Seconds += sample.Seconds
			missingSamples++
		}
		previous = sample
	}
	finishAbsence()
	return summary
}

func sumPresenceResponse(response *types.PresenceResponse) {
	for _, day := range response.Days {
		response.PresentSeconds += day.PresentSeconds
		response.AbsentSeconds += day.AbsentSeconds
		response.UnavailableSeconds += day.UnavailableSeconds
		response.AbsenceCount += day.AbsenceCount
		if day.LongestAbsence > response.LongestAbsence {
			response.LongestAbsence = day.LongestAbsence
		}
	}
}

func (s *StatsStorage) NearestPresenceShift(period types.ActivityPeriod, fromShift int, older bool) (int, bool) {
	now := time.Now()
	currentWeek := activityWeekStart(now)
	best := -1
	for day := range s.presenceDays() {
		at, err := time.ParseInLocation(TruncatedToDay, day, time.Local)
		if err != nil {
			continue
		}
		shift := int(activityDayIndex(now) - activityDayIndex(at))
		if period == types.ActivityWeekly {
			shift = int((activityDayIndex(currentWeek) - activityDayIndex(activityWeekStart(at))) / 7)
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
