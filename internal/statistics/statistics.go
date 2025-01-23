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

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Stop received, finishing Statistics handling")
			storage.IncreaseStatistics(activeApplication, activatedAt)
			fmt.Println("Storage closed")
			wg := ctx.Value(types.WgKey{}).(*sync.WaitGroup)
			wg.Done()
			return

		case <-time.Tick(time.Second * 30):
			fmt.Printf("Active Application: %s\n", activeApplication)
			activatedAt = storage.IncreaseStatistics(activeApplication, activatedAt)
			storage.DumpTheUsage()

		case command := <-commandsChannel:
			switch command.Type() {
			case types.Command:
				request := command.(types.RequestCommand)
				activatedAt = storage.IncreaseStatistics(activeApplication, activatedAt)
				request.ResponseChan <- storage.GetStatisticsCurrentHour()

			case types.Event:
				event := command.(types.NewAppEvent)
				activatedAt = storage.IncreaseStatistics(activeApplication, activatedAt)
				fmt.Printf("Active Application changed: %s -> %s\n", activeApplication, event.AppName)
				activeApplication = event.AppName
			}
		}
	}
}
