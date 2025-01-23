package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"parental-control/internal/lib/types"
	"sync"
	"time"

	"github.com/txn2/txeh"
	tele "gopkg.in/telebot.v4"
	"gopkg.in/telebot.v4/middleware"
)

var (
	selector   = &tele.ReplyMarkup{}
	b30Minutes = selector.Data("30 Minutes", "30-minutes")
	b1Hour     = selector.Data("1 Hour", "1-hour")
	bClose     = selector.Data("Close", "close")
)

func StartBot(ctx context.Context, requests chan<- types.AppCommand) {
	fmt.Println("Running bot")
	var timersCtx context.Context
	var timersCancelFunc context.CancelFunc

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

	selector.Inline(
		selector.Row(b30Minutes, b1Hour, bClose),
	)

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

	b.Handle("/youtube", func(c tele.Context) error {
		return c.Reply("For how long?", selector)
	})

	b.Handle(&b30Minutes, func(c tele.Context) error {
		fmt.Println("Youtube open for 30 minutes request received")
		// c.Respond()
		if timersCancelFunc != nil {
			timersCancelFunc()
		}
		timersCtx, timersCancelFunc = context.WithCancel(ctx)
		go startYoutubeTimer(c, timersCtx, 30*time.Minute)
		return c.Edit("Timer for 30 minutes was set")
	})

	b.Handle(&b1Hour, func(c tele.Context) error {
		fmt.Println("Youtube open for 1 hour request received")
		// c.Respond()
		if timersCancelFunc != nil {
			timersCancelFunc()
		}
		timersCtx, timersCancelFunc = context.WithCancel(ctx)
		go startYoutubeTimer(c, timersCtx, 1*time.Hour)
		return c.Edit("Timer for 1 hour was set")
	})

	b.Handle(&bClose, func(c tele.Context) error {
		fmt.Println("Youtube close request received")
		// c.Respond()
		if timersCancelFunc != nil {
			timersCancelFunc()
		}
		return c.Edit("Youtube closed")
	})

	// b.Handle(tele.OnInlineResult, func(c tele.Context) error {
	// 	fmt.Println("Inline btn received")
	// 	go func(duration int) {
	// 		fmt.Println("Starting timer for 30 seconds")
	// 		<-time.Tick(time.Second * 30)
	// 		c.Reply("30 seconds timer finished...")
	// 	}(30)

	// 	return c.Edit("Zhopa")
	// })

	b.Start()
}

func startYoutubeTimer(c tele.Context, timersCtx context.Context, duration time.Duration) {
	var hosts = timersCtx.Value(types.HostsKey{}).(*txeh.Hosts)
	hosts.RemoveHosts([]string{"youtube.com", "www.youtube.com"})
	hfData := hosts.RenderHostsFile()
	fmt.Println(hfData)
	err := hosts.Save()
	if err != nil {
		fmt.Printf("Failed to update /etc/hosts: %s\n", err.Error())
	}

	fmt.Printf("Starting timer for %s\n", duration.String())
	select {
	case <-timersCtx.Done():
		fmt.Printf("Cancelling timer for %s\n", duration.String())
	case <-time.Tick(duration):
		c.Reply(fmt.Sprintf("%s timer finished...", duration.String()))
	}

	hosts.AddHosts("127.0.0.1", []string{"youtube.com", "www.youtube.com"})
	hfData = hosts.RenderHostsFile()
	fmt.Println(hfData)
	err = hosts.Save()
	if err != nil {
		fmt.Printf("Failed to update /etc/hosts: %s\n", err.Error())
	}
}
