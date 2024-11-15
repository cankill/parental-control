package main

import (
	"fmt"
	"time"

	"github.com/cankill/parental-control/internal/bot"
	"github.com/cankill/parental-control/internal/tools"
	"github.com/progrium/darwinkit/macos"
	"github.com/progrium/darwinkit/macos/appkit"
	"github.com/progrium/darwinkit/macos/foundation"
)

var handler = func(activeApp string, appActivated <-chan string, requests chan<- []tools.AppInfo) {
	fmt.Println("Running handler")
	applications := map[string]int64{}
	activeApplication := activeApp
	activatedAt := time.Now().UnixMilli()
	var statistics []tools.AppInfo

	calculateStatistics := func() {
		result := make([]tools.AppInfo, 0)
		for app, time := range applications {
			result = append(result, tools.AppInfo{Identity: app, Time: time})
		}
		statistics = result
	}

	for {
		select {
		case requests <- statistics:
			// fmt.Println("Statistics sent")
			// for _, ai := range statistics {
			// 	fmt.Print(ai.Dump())
			// }
		case newActiveApplication := <-appActivated:
			now := time.Now().UnixMilli()
			periodAppWasActive := now - activatedAt
			applications[activeApplication] = applications[activeApplication] + periodAppWasActive
			activeApplication = newActiveApplication
			activatedAt = now

			// fmt.Println("Applications:")
			// for app, time := range applications {
			// 	fmt.Printf("App name: %s, time: %d\n", app, time)
			// }

			calculateStatistics()
		}
	}
}

// var bundleIdentity = foundation.NewStringWithString("NSApplicationBundleIdentifier")
var appKey = foundation.NewStringWithString("NSWorkspaceApplicationKey")

func main() {
	notifications := make(chan string)
	defer close(notifications)
	requests := make(chan []tools.AppInfo)
	defer close(requests)

	macos.RunApp(func(app appkit.Application, delegate *appkit.ApplicationDelegate) {
		fmt.Println("Let's start application")
		ws := appkit.Workspace_SharedWorkspace()

		frontmost := ws.FrontmostApplication()

		go handler(frontmost.BundleIdentifier(), notifications, requests)
		go bot.StartBot(requests)

		notificationCenter := ws.NotificationCenter()
		notificationCenter.AddObserverForNameObjectQueueUsingBlock(
			"NSWorkspaceDidActivateApplicationNotification",
			nil,
			foundation.OperationQueue_MainQueue(),
			func(notification foundation.Notification) {
				focussedApp := appkit.RunningApplicationFrom(notification.UserInfo().ObjectForKey(appKey).Ptr()).BundleIdentifier()
				notifications <- focussedApp
			})
	})
}
