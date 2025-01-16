package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"parental-control/internal/lib/types"
	"strings"
	"sync"
	"time"

	tele "gopkg.in/telebot.v4"
	"gopkg.in/telebot.v4/middleware"
)

func StartBot(ctx context.Context, requests chan<- types.AppCommand) {
	fmt.Println("Running bot")
	index := 0
	pref := tele.Settings{
		Token:  os.Getenv("BOT_TOKEN"),
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
		return
	}

	go func() {
		<-ctx.Done()
		b.Stop()
		fmt.Println("Bot stopped")
		wg := ctx.Value(types.WgKey{}).(*sync.WaitGroup)
		wg.Done()
	}()

	admins := []int64{183358896}

	b.Use(middleware.Whitelist(admins...))

	// b.Use(middleware.Logger())
	// b.Use(middleware.AutoRespond())

	// Command: /start <PAYLOAD>
	b.Handle("/youtube", func(c tele.Context) error {
		args := strings.Split(c.Message().Payload, " ")
		if len(args) != 1 {
			c.Send("Error1")
			return c.Send("Error2")
		} else {
			fmt.Println(c.Message().Payload) // <PAYLOAD>
			return c.Send("Ok")
		}
	})

	b.Handle("/ping", func(c tele.Context) error {
		return c.Send("pong")
	})

	b.Handle("/status", func(c tele.Context) error {
		responseChan := make(chan types.AppInfos)
		requests <- types.RequestCommand{ResponseChan: responseChan}
		appInfos := <-responseChan

		appInfos.SortByDurationDesc()
		statistics := "```\n" + appInfos.FormatTable() + "\n```"

		return c.Send(statistics, &tele.SendOptions{
			ParseMode: tele.ModeMarkdownV2,
		})
	})

	// Command: /start <PAYLOAD>
	b.Handle("/screen", func(c tele.Context) error {
		fname := fmt.Sprintf("/tmp/pc/capture-%d.png", index)
		cmd := exec.Command("/usr/sbin/screencapture", fname)
		if err := cmd.Run(); err != nil {
			fmt.Println("Error: ", err)
			return c.Send(fmt.Sprintf("Error : %s", err))
		}

		image := &tele.Photo{File: tele.FromDisk(fname)}

		return c.Send(image)
	})

	b.Start()
}
