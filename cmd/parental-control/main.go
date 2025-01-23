package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"parental-control/internal/bot"
	"parental-control/internal/lib/types"
	"parental-control/internal/statistics"
	"sync"
	"syscall"

	"github.com/progrium/darwinkit/macos"
	"github.com/progrium/darwinkit/macos/appkit"
	"github.com/progrium/darwinkit/macos/foundation"
	"github.com/txn2/txeh"
)

var appKey = foundation.NewStringWithString("NSWorkspaceApplicationKey")

func main() {
	macos.RunApp(func(app appkit.Application, delegate *appkit.ApplicationDelegate) {
		// env := config.MustLoad()
		// Open /etc/hosts file for managing
		hosts, err := txeh.NewHostsDefault()
		if err != nil {
			panic(err)
		}

		wg := sync.WaitGroup{}
		ctx, cancelFunc := context.WithCancel(context.Background())
		ctx = context.WithValue(ctx, types.WgKey{}, &wg)
		ctx = context.WithValue(ctx, types.HostsKey{}, hosts)
		// ctx = context.WithValue(ctx, types.EnvKey{}, env)

		statisticsCommandsChannel := make(chan types.AppCommand)
		sigs := make(chan os.Signal, 1)

		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		fmt.Println("Starting up Mac OS X Application")
		sharedWorkspace := appkit.Workspace_SharedWorkspace()
		initiallyActiveApplication := sharedWorkspace.FrontmostApplication()

		wg.Add(1)
		go statistics.Handler(ctx, initiallyActiveApplication.BundleIdentifier(), statisticsCommandsChannel)

		wg.Add(1)
		go bot.StartBot(ctx, statisticsCommandsChannel)

		go func() {
			<-sigs
			fmt.Println()
			fmt.Println("Stopping Mac OS X Application")

			cancelFunc()

			close(sigs)
			close(statisticsCommandsChannel)

			wg.Wait()
			app.Terminate(app)
		}()

		notificationCenter := sharedWorkspace.NotificationCenter()
		notificationCenter.AddObserverForNameObjectQueueUsingBlock(
			"NSWorkspaceDidActivateApplicationNotification",
			nil,
			foundation.OperationQueue_MainQueue(),
			func(notification foundation.Notification) {
				focussedApp := appkit.RunningApplicationFrom(notification.UserInfo().ObjectForKey(appKey).Ptr()).BundleIdentifier()
				statisticsCommandsChannel <- types.NewAppEvent{AppName: focussedApp}
			})

		fmt.Println("Mac OS X Application started")
	})
}
