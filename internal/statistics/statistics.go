package statistics

import (
	"context"
	"fmt"
	"log"
	"parental-control/internal/appinfo"
	"parental-control/internal/lib/types"
	"parental-control/internal/statistics/statstorage"
	"strings"
	"sync"
	"time"
)

const slowStatisticsQueryThreshold = 50 * time.Millisecond

func logStatisticsQuery(kind string, shift int, started time.Time, cache statstorage.CacheMetrics) {
	elapsed := time.Since(started)
	if elapsed < slowStatisticsQueryThreshold {
		return
	}
	log.Printf("Slow statistics query: kind=%s shift=%d duration=%s cache_hits=%d cache_misses=%d cache_entries=%d",
		kind, shift, elapsed.Round(time.Millisecond), cache.Hits, cache.Misses, cache.Entries)
}

func activityPeriodName(period types.ActivityPeriod) string {
	switch period {
	case types.ActivityDaily:
		return "daily"
	case types.ActivityWeekly:
		return "weekly"
	default:
		return "hourly"
	}
}

// formatAppInfo превращает результаты поиска по словарю в читаемый текст для /info.
func formatAppInfo(name string, infos []appinfo.Info) string {
	if len(infos) == 0 {
		return fmt.Sprintf("No info for %q yet (tracked apps only).", name)
	}
	var b strings.Builder
	for i, info := range infos {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%s\n", statstorage.DisplayName(info.BundleID))
		fmt.Fprintf(&b, "  bundle: %s\n", info.BundleID)
		if info.Name != "" {
			fmt.Fprintf(&b, "  name:   %s\n", info.Name)
		}
		if info.Version != "" {
			fmt.Fprintf(&b, "  ver:    %s\n", info.Version)
		}
		if info.Path != "" {
			fmt.Fprintf(&b, "  path:   %s\n", info.Path)
		}
	}
	return b.String()
}

func Handler(ctx context.Context, activeApplication string, commandsChannel <-chan types.AppCommand) {
	fmt.Println("Running handler")
	activatedAt := time.Now()

	storage := statstorage.Open()
	fmt.Println("Storage opened")

	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Stop received, finishing Statistics handling")
			for {
				select {
				case command := <-commandsChannel:
					switch command.Type() {
					case types.ActivityEvent:
						storage.AddActivity(command.(types.ActivityBatch).Samples)
					case types.PresenceEvent:
						storage.AddPresenceSample(command.(types.PresenceSample))
					}
				default:
					goto drained
				}
			}
		drained:
			storage.IncreaseStatistics(activeApplication, activatedAt)
			fmt.Println("Storage closed")
			wg := ctx.Value(types.WgKey{}).(*sync.WaitGroup)
			wg.Done()
			return

		case <-ticker.C:
			fmt.Printf("Active Application: %s\n", activeApplication)
			activatedAt = storage.IncreaseStatistics(activeApplication, activatedAt)
			storage.DumpTheUsage()

		case command := <-commandsChannel:
			switch command.Type() {
			case types.Command:
				started := time.Now()
				request := command.(types.RequestCommand)
				activatedAt = storage.IncreaseStatistics(activeApplication, activatedAt)
				resp := storage.GetStatisticsShifted(request.ShiftHours)
				resp.ShiftHours = request.ShiftHours
				resp.OlderShift, resp.HasOlder = storage.NearestShift(request.ShiftHours, true)
				resp.NewerShift, resp.HasNewer = storage.NearestShift(request.ShiftHours, false)
				if request.ShiftHours == 0 && statstorage.ShouldTrackApplication(activeApplication) {
					resp.ActiveApp = statstorage.DisplayName(activeApplication)
				}
				logStatisticsQuery("apps-hourly", request.ShiftHours, started, storage.CacheMetrics())
				request.ResponseChan <- resp

			case types.DayCommand:
				started := time.Now()
				request := command.(types.DayRequest)
				activatedAt = storage.IncreaseStatistics(activeApplication, activatedAt)
				resp := storage.GetStatisticsDay(request.DayShift)
				resp.OlderShift, resp.HasOlder = storage.NearestDayShift(request.DayShift, true)
				resp.NewerShift, resp.HasNewer = storage.NearestDayShift(request.DayShift, false)
				logStatisticsQuery("apps-daily", request.DayShift, started, storage.CacheMetrics())
				request.ResponseChan <- resp

			case types.WeekCommand:
				started := time.Now()
				request := command.(types.WeekRequest)
				activatedAt = storage.IncreaseStatistics(activeApplication, activatedAt)
				resp := storage.GetStatisticsWeek(request.WeekShift)
				resp.OlderShift, resp.HasOlder = storage.NearestWeekShift(request.WeekShift, true)
				resp.NewerShift, resp.HasNewer = storage.NearestWeekShift(request.WeekShift, false)
				logStatisticsQuery("apps-weekly", request.WeekShift, started, storage.CacheMetrics())
				request.ResponseChan <- resp

			case types.DomainCommand:
				started := time.Now()
				request := command.(types.DomainRequest)
				var resp *types.AppInfoResponse
				switch request.Period {
				case types.ActivityDaily:
					resp = storage.GetDomainStatisticsDay(request.ShiftHours)
					resp.OlderShift, resp.HasOlder = storage.NearestDomainDayShift(request.ShiftHours, true)
					resp.NewerShift, resp.HasNewer = storage.NearestDomainDayShift(request.ShiftHours, false)
				case types.ActivityWeekly:
					resp = storage.GetDomainStatisticsWeek(request.ShiftHours)
					resp.OlderShift, resp.HasOlder = storage.NearestDomainWeekShift(request.ShiftHours, true)
					resp.NewerShift, resp.HasNewer = storage.NearestDomainWeekShift(request.ShiftHours, false)
				default:
					resp = storage.GetDomainStatistics(request.ShiftHours)
					resp.OlderShift, resp.HasOlder = storage.NearestDomainShift(request.ShiftHours, true)
					resp.NewerShift, resp.HasNewer = storage.NearestDomainShift(request.ShiftHours, false)
				}
				logStatisticsQuery("sites-"+activityPeriodName(request.Period), request.ShiftHours, started, storage.CacheMetrics())
				request.ResponseChan <- resp

			case types.DomainEvent:
				tick := alignDomainTick(command.(types.DomainTick), activeApplication, activatedAt)
				storage.AddDomainSample(activeApplication, tick)

			case types.AppInfoCommand:
				query := command.(types.AppInfoQuery)
				query.ResponseChan <- formatAppInfo(query.Name, storage.FindAppInfoByName(query.Name))

			case types.ActivityEvent:
				storage.AddActivity(command.(types.ActivityBatch).Samples)

			case types.ActivityCommand:
				started := time.Now()
				request := command.(types.ActivityRequest)
				resp := storage.GetActivity(request.Period, request.Shift)
				resp.OlderShift, resp.HasOlder = storage.NearestActivityShift(request.Period, request.Shift, true)
				resp.NewerShift, resp.HasNewer = storage.NearestActivityShift(request.Period, request.Shift, false)
				logStatisticsQuery("activity-"+activityPeriodName(request.Period), request.Shift, started, storage.CacheMetrics())
				request.ResponseChan <- resp

			case types.PresenceEvent:
				storage.AddPresenceSample(command.(types.PresenceSample))

			case types.PresenceCommand:
				started := time.Now()
				request := command.(types.PresenceRequest)
				resp := storage.GetPresence(request.Period, request.Shift)
				resp.OlderShift, resp.HasOlder = storage.NearestPresenceShift(request.Period, request.Shift, true)
				resp.NewerShift, resp.HasNewer = storage.NearestPresenceShift(request.Period, request.Shift, false)
				logStatisticsQuery("presence-"+activityPeriodName(request.Period), request.Shift, started, storage.CacheMetrics())
				request.ResponseChan <- resp

			case types.Event:
				event := command.(types.NewAppEvent)
				activatedAt = storage.IncreaseStatistics(activeApplication, activatedAt)
				fmt.Printf("Active Application changed: %s -> %s\n", activeApplication, event.AppName)
				activeApplication = event.AppName
				storage.RememberApp(activeApplication) // словарь для /info (резолв один раз)
			}
		}
	}
}

// alignDomainTick prevents the first poll after an application switch from
// assigning time from before the browser became active to the new domain.
func alignDomainTick(tick types.DomainTick, activeApplication string, activatedAt time.Time) types.DomainTick {
	if tick.At.IsZero() || !strings.EqualFold(activeApplication, tick.BrowserBundleID) {
		return tick
	}
	activeMillis := tick.At.Sub(activatedAt).Milliseconds()
	if activeMillis < 0 {
		activeMillis = 0
	}
	if tick.Millis > activeMillis {
		tick.Millis = activeMillis
	}
	return tick
}
