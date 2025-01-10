package main

import (
	"fmt"
	"parental-control/internal/bot"
	"parental-control/internal/statistics"
	"parental-control/internal/tools"

	"github.com/progrium/darwinkit/macos"
	"github.com/progrium/darwinkit/macos/appkit"
	"github.com/progrium/darwinkit/macos/foundation"
)

var appKey = foundation.NewStringWithString("NSWorkspaceApplicationKey")

func main() {
	// sigs := make(chan os.Signal, 1)

	notifications := make(chan string)
	defer close(notifications)
	requests := make(chan tools.Request)
	defer close(requests)

	macos.RunApp(func(app appkit.Application, delegate *appkit.ApplicationDelegate) {
		// signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		fmt.Println("Starting up Mac OS X Application")
		sharedWorkspace := appkit.Workspace_SharedWorkspace()
		initiallyActiveApplication := sharedWorkspace.FrontmostApplication()

		go statistics.Handler(initiallyActiveApplication.BundleIdentifier(), notifications, requests)
		go bot.StartBot(requests)
		// go func() {
		// 	sig := <-sigs
		// 	fmt.Println()
		// 	fmt.Printf("Signal received: %v", sig)
		// 	fmt.Println("Stopping Mac OS X Application")
		// }()

		notificationCenter := sharedWorkspace.NotificationCenter()
		notificationCenter.AddObserverForNameObjectQueueUsingBlock(
			"NSWorkspaceDidActivateApplicationNotification",
			nil,
			foundation.OperationQueue_MainQueue(),
			func(notification foundation.Notification) {
				focussedApp := appkit.RunningApplicationFrom(notification.UserInfo().ObjectForKey(appKey).Ptr()).BundleIdentifier()
				notifications <- focussedApp
			})

		fmt.Println("Mac OS X Application started")
	})
}
