package main

import (
	"fmt"
	"os"
	"os/signal"
	"parental-control/internal/bot"
	"parental-control/internal/lib/types"
	"parental-control/internal/statistics"
	"syscall"

	"github.com/progrium/darwinkit/macos"
	"github.com/progrium/darwinkit/macos/appkit"
	"github.com/progrium/darwinkit/macos/foundation"
)

var appKey = foundation.NewStringWithString("NSWorkspaceApplicationKey")

func main() {
	commandsChannel := make(chan types.AppCommand)
	defer close(commandsChannel)
	requests := make(chan types.Request)
	defer close(requests)

	macos.RunApp(func(app appkit.Application, delegate *appkit.ApplicationDelegate) {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		fmt.Println("Starting up Mac OS X Application")
		sharedWorkspace := appkit.Workspace_SharedWorkspace()
		initiallyActiveApplication := sharedWorkspace.FrontmostApplication()

		go statistics.Handler(initiallyActiveApplication.BundleIdentifier(), commandsChannel, requests)
		go bot.StartBot(requests)
		go func() {
			<-sigs
			fmt.Println()
			fmt.Println("Stopping Mac OS X Application")
			stoppedChan := make(chan bool)
			commandsChannel <- types.StopCommand{StoppedChan: stoppedChan}
			<-stoppedChan
			app.Terminate(app)
		}()

		notificationCenter := sharedWorkspace.NotificationCenter()
		notificationCenter.AddObserverForNameObjectQueueUsingBlock(
			"NSWorkspaceDidActivateApplicationNotification",
			nil,
			foundation.OperationQueue_MainQueue(),
			func(notification foundation.Notification) {
				focussedApp := appkit.RunningApplicationFrom(notification.UserInfo().ObjectForKey(appKey).Ptr()).BundleIdentifier()
				commandsChannel <- types.NewAppEvent{AppName: focussedApp}
			})

		fmt.Println("Mac OS X Application started")
	})
}
