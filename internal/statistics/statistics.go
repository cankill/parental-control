package statistics

import (
	"fmt"
	"parental-control/internal/lib/types"
	"time"
)

func Handler(activeApplication string, commandsChannel <-chan types.AppCommand, requests <-chan types.Request) {
	//TODO: Load Satistics from DB
	defer func() {
		fmt.Println("Todo: store statistics to DB")
	}()

	fmt.Println("Running handler")
	applications := map[string]int64{}
	activatedAt := time.Now().UnixMilli()

	statistics := func() []types.AppInfo {
		statistics := make([]types.AppInfo, 0)
		for app, time := range applications {
			statistics = append(statistics, types.AppInfo{Identity: app, Time: time})
		}
		return statistics
	}

	for {
		select {
		case request := <-requests:
			now := time.Now().UnixMilli()
			periodAppWasActive := now - activatedAt
			applications[activeApplication] = applications[activeApplication] + periodAppWasActive
			activatedAt = now
			request.ResponseChan <- statistics()
		case command := <-commandsChannel:
			switch command.Type() {
			case types.Command:
				fmt.Println("Stop received, finishing Statistics handling")
				return
			case types.Event:
				event := command.(types.NewAppEvent)
				now := time.Now().UnixMilli()
				periodAppWasActive := now - activatedAt
				applications[activeApplication] = applications[activeApplication] + periodAppWasActive
				activeApplication = event.AppName
				activatedAt = now
			}
		}
	}
}
