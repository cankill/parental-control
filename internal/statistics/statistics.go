package statistics

import (
	"context"
	"fmt"
	"parental-control/internal/lib/types"
	"parental-control/internal/statistics/statstorage"
	"sync"
	"time"
)

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
				request.ResponseChan <- resp

			case types.PeriodCommand:
				request := command.(types.PeriodRequest)
				activatedAt = storage.IncreaseStatistics(activeApplication, activatedAt)
				request.ResponseChan <- storage.GetStatisticsPeriod(request.FromShift, request.ToShift)

			case types.Event:
				event := command.(types.NewAppEvent)
				activatedAt = storage.IncreaseStatistics(activeApplication, activatedAt)
				fmt.Printf("Active Application changed: %s -> %s\n", activeApplication, event.AppName)
				activeApplication = event.AppName
			}
		}
	}
}
