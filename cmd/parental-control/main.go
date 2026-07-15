package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"parental-control/internal/bot"
	"parental-control/internal/lib/config"
	"parental-control/internal/lib/types"
	"parental-control/internal/statistics"
	"sync"
	"syscall"

	"github.com/progrium/darwinkit/macos"
	"github.com/progrium/darwinkit/macos/appkit"
	"github.com/progrium/darwinkit/macos/foundation"
)

var appKey = foundation.NewStringWithString("NSWorkspaceApplicationKey")

func main() {
	macos.RunApp(func(app appkit.Application, delegate *appkit.ApplicationDelegate) {
		env := config.MustLoad()

		// Блокировка /etc/hosts вынесена в privileged helper (LaunchDaemon, root);
		// основное приложение обращается к нему через сокет из пакета bot.
		wg := sync.WaitGroup{}
		ctx, cancelFunc := context.WithCancel(context.Background())
		ctx = context.WithValue(ctx, types.WgKey{}, &wg)
		ctx = context.WithValue(ctx, types.EnvKey{}, env)

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

		wg.Add(1)
		go statistics.TrackDomains(ctx, env.UrlPollInterval(), statisticsCommandsChannel)

		go func() {
			<-sigs
			fmt.Println()
			fmt.Println("Stopping Mac OS X Application")

			// Отменяем контекст и дожидаемся завершения обработчиков. Канал
			// statisticsCommandsChannel НЕ закрываем: observer ниже продолжает
			// писать в него до отмены контекста, а закрытие со стороны отправителя
			// привело бы к панике "send on closed channel".
			cancelFunc()

			close(sigs)

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
				// После отмены контекста получатель уже завершился — не пишем в канал.
				select {
				case statisticsCommandsChannel <- types.NewAppEvent{AppName: focussedApp}:
				case <-ctx.Done():
				}
			})

		fmt.Println("Mac OS X Application started")
	})
}
