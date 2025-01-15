package statistics

import (
	"fmt"
	"os"
	"parental-control/internal/lib/storage"
	"parental-control/internal/lib/types"
	"time"
)

func Handler(activeApplication string, commandsChannel <-chan types.AppCommand, requests <-chan types.Request) {
	fmt.Println("Running handler")
	activatedAt := time.Now()
	activeBucketName := ""

	storage, err := storage.New()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("Storage opened")

	for {
		select {
		case request := <-requests:
			activeBucketName, activatedAt = storage.IncreaseStatistics(activeBucketName, activeApplication, activatedAt)
			request.ResponseChan <- storage.GetStatisticsCurrentHour()

		case command := <-commandsChannel:
			switch command.Type() {
			case types.Command:
				fmt.Println("Stop received, finishing Statistics handling")
				storage.IncreaseStatistics(activeBucketName, activeApplication, activatedAt)
				storage.Close()
				fmt.Println("Storage closed")
				stopCommand := command.(types.StopCommand)
				stopCommand.StoppedChan <- true
				return

			case types.Event:
				event := command.(types.NewAppEvent)
				activeBucketName, activatedAt = storage.IncreaseStatistics(activeBucketName, activeApplication, activatedAt)
				activeApplication = event.AppName
			}
		}
	}
}
