package main

import (
	"fmt"
	"log"
	"time"

	"github.com/progrium/darwinkit/macos"
	"github.com/progrium/darwinkit/macos/appkit"
	"github.com/progrium/darwinkit/macos/foundation"
)

type ActionType string

const (
	APPLICATION_STARTED  ActionType = "Application Started"
	APPLICATION_FINISHED            = "Application Finished"
	GAIN_FOCUS                      = "Application Gained Focus"
	LOOSE_FOCUS                     = "Application Looses Focus"
)

type AppAction struct {
	identity string
	action   ActionType
}

func (ac AppAction) dump() string {
	return fmt.Sprintf("AppAction {identity: %s, action: %s}", ac.identity, ac.action)
}

var handler = func(appActions <-chan AppAction, requests chan<- AppAction) {
	applications := map[int64]AppAction{}
	fmt.Println("Running handler:")
	latestNotification := AppAction{"", GAIN_FOCUS}
	for {
		select {
		case requests <- latestNotification:
			fmt.Println("in request")
		case action := <-appActions:
			fmt.Println(action.dump())
			if action.action == GAIN_FOCUS {
				latestNotification = action
			}
			applications[time.Now().UnixMilli()] = action
		}
		// time.Sleep(time.Millisecond * 300)
	}
}

var bundleIdentity = foundation.NewStringWithString("NSApplicationBundleIdentifier")
var appKey = foundation.NewStringWithString("NSWorkspaceApplicationKey")

func main() {
	notifications := make(chan AppAction)
	defer close(notifications)
	requests := make(chan AppAction)
	defer close(requests)
	go handler(notifications, requests)
	go startBot(requests)

	macos.RunApp(func(app appkit.Application, delegate *appkit.ApplicationDelegate) {
		fmt.Println("Let's start application")
		ws := appkit.Workspace_SharedWorkspace()
		notificationCenter := ws.NotificationCenter()

		notificationCenter.AddObserverForNameObjectQueueUsingBlock(
			"NSWorkspaceDidLaunchApplicationNotification",
			nil,
			foundation.OperationQueue_MainQueue(),
			func(notification foundation.Notification) {
				log.Println("Launched")
				appId := notification.UserInfo().ObjectForKey(bundleIdentity).Description()
				notifications <- AppAction{appId, APPLICATION_STARTED}
			})

		notificationCenter.AddObserverForNameObjectQueueUsingBlock(
			"NSWorkspaceDidActivateApplicationNotification",
			nil,
			foundation.OperationQueue_MainQueue(),
			func(notification foundation.Notification) {
				log.Println("Focused")
				runningApp := appkit.RunningApplicationFrom(notification.UserInfo().ObjectForKey(appKey).Ptr()).BundleIdentifier()
				notifications <- AppAction{runningApp, GAIN_FOCUS}
			})

		notificationCenter.AddObserverForNameObjectQueueUsingBlock(
			"NSWorkspaceDidDeactivateApplicationNotification",
			nil,
			foundation.OperationQueue_MainQueue(),
			func(notification foundation.Notification) {
				log.Println("Focus loose")
				runningApp := appkit.RunningApplicationFrom(notification.UserInfo().ObjectForKey(appKey).Ptr()).BundleIdentifier()
				notifications <- AppAction{runningApp, LOOSE_FOCUS}
			})

		fmt.Println("Listeners activated")
	})
}
