package statistics

import (
	"context"
	"fmt"
	"parental-control/internal/appinfo"
	"parental-control/internal/lib/types"
	"parental-control/internal/statistics/statstorage"
	"strings"
	"sync"
	"time"
)

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
					if command.Type() == types.ActivityEvent {
						storage.AddActivity(command.(types.ActivityBatch).Samples)
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
				request := command.(types.RequestCommand)
				activatedAt = storage.IncreaseStatistics(activeApplication, activatedAt)
				resp := storage.GetStatisticsShifted(request.ShiftHours)
				resp.ShiftHours = request.ShiftHours
				resp.OlderShift, resp.HasOlder = storage.NearestShift(request.ShiftHours, true)
				resp.NewerShift, resp.HasNewer = storage.NearestShift(request.ShiftHours, false)
				if request.ShiftHours == 0 {
					resp.ActiveApp = statstorage.DisplayName(activeApplication)
				}
				request.ResponseChan <- resp

			case types.DayCommand:
				request := command.(types.DayRequest)
				activatedAt = storage.IncreaseStatistics(activeApplication, activatedAt)
				resp := storage.GetStatisticsDay(request.DayShift)
				resp.OlderShift, resp.HasOlder = storage.NearestDayShift(request.DayShift, true)
				resp.NewerShift, resp.HasNewer = storage.NearestDayShift(request.DayShift, false)
				request.ResponseChan <- resp

			case types.DomainCommand:
				request := command.(types.DomainRequest)
				resp := storage.GetDomainStatistics(request.ShiftHours)
				resp.OlderShift, resp.HasOlder = storage.NearestDomainShift(request.ShiftHours, true)
				resp.NewerShift, resp.HasNewer = storage.NearestDomainShift(request.ShiftHours, false)
				request.ResponseChan <- resp

			case types.DomainEvent:
				tick := command.(types.DomainTick)
				storage.AddDomainTime(tick.Domain, tick.Millis)

			case types.AppInfoCommand:
				query := command.(types.AppInfoQuery)
				query.ResponseChan <- formatAppInfo(query.Name, storage.FindAppInfoByName(query.Name))

			case types.ActivityEvent:
				storage.AddActivity(command.(types.ActivityBatch).Samples)

			case types.ActivityCommand:
				request := command.(types.ActivityRequest)
				resp := storage.GetActivity(request.ShiftHours)
				resp.OlderShift, resp.HasOlder = storage.NearestActivityShift(request.ShiftHours, true)
				resp.NewerShift, resp.HasNewer = storage.NearestActivityShift(request.ShiftHours, false)
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
