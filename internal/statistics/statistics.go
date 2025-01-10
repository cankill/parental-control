package statistics

import (
	"fmt"
	"parental-control/internal/tools"
	"time"
)

func Handler(activeApplication string, appActivated <-chan string, requests <-chan tools.Request) {
	defer func() {
		fmt.Println("Handler finished")
	}()

	fmt.Println("Running handler")
	applications := map[string]int64{}
	activatedAt := time.Now().UnixMilli()
	var statistics []tools.AppInfo

	calculateStatistics := func() {
		newStatistics := make([]tools.AppInfo, 0)
		for app, time := range applications {
			newStatistics = append(newStatistics, tools.AppInfo{Identity: app, Time: time})
		}
		statistics = newStatistics
	}

	for {
		select {
		case request := <-requests:
			now := time.Now().UnixMilli()
			periodAppWasActive := now - activatedAt
			applications[activeApplication] = applications[activeApplication] + periodAppWasActive
			activatedAt = now
			calculateStatistics()
			request.ResponseChan <- statistics
		case newActiveApplication := <-appActivated:
			now := time.Now().UnixMilli()
			periodAppWasActive := now - activatedAt
			applications[activeApplication] = applications[activeApplication] + periodAppWasActive
			activeApplication = newActiveApplication
			activatedAt = now
			calculateStatistics()
		}
	}
}
