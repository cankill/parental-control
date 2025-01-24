package bot

import (
	"context"
	"fmt"
	"log"
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
	bBlock     = selector.Data("Block", "block")
	bUnblock   = selector.Data("Unblock", "un-block")
	BOT_TOKEN  = "8180855001:AAEBlkFMvDxN3I9fQ2vm7m6wf2yGSSgWf70"
	cankill    = int64(183358896)
	admins     = []int64{cankill}
)

func StartBot(ctx context.Context, requests chan<- types.AppCommand) {
	fmt.Println("Running bot")
	timersCtx, timersCancelFunc := context.WithCancel(ctx)

	index := 0
	pref := tele.Settings{
		Token:  BOT_TOKEN,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
		return
	}

	selector.Inline(
		selector.Row(b30Minutes, b1Hour, bBlock, bUnblock),
	)

	go func() {
		<-ctx.Done()
		b.Stop()
		fmt.Println("Bot stopped")
		wg := ctx.Value(types.WgKey{}).(*sync.WaitGroup)
		wg.Done()
	}()

	b.Use(middleware.Whitelist(admins...))
	// b.Use(middleware.Logger())

	b.Handle("/status", func(c tele.Context) error {
		responseChan := make(chan *types.AppInfoResponse)
		requests <- types.RequestCommand{ResponseChan: responseChan, Shift: 0}
		statisticsResponse := <-responseChan

		statisticsResponse.AppInfos.SortByDurationDesc()
		statisticsTable := "```\n" + "  For: " + statisticsResponse.TimeStamp + "\n\n" + statisticsResponse.AppInfos.FormatTable() + "\n```"
		keyboard := makeStatisticsKeyboard("aaa", "")
		return c.Send(statisticsTable, &tele.SendOptions{ParseMode: tele.ModeMarkdownV2, ReplyMarkup: keyboard})
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
		if timersCancelFunc != nil {
			timersCancelFunc()
		}
		timersCtx, timersCancelFunc = context.WithCancel(ctx)
		go startYoutubeTimer(c, timersCtx, 30*time.Minute,
			func() {
				blockHosts(timersCtx, "127.0.0.1", "youtube.com", "www.youtube.com")
			},
			func() {
				unblockHosts(timersCtx, "youtube.com", "www.youtube.com")
			})
		c.Edit("Timer for 30 minutes was set")
		return c.Respond()
	})

	b.Handle(&b1Hour, func(c tele.Context) error {
		fmt.Println("Youtube open for 1 hour request received")
		if timersCancelFunc != nil {
			timersCancelFunc()
		}
		timersCtx, timersCancelFunc = context.WithCancel(ctx)
		go startYoutubeTimer(c, timersCtx, 1*time.Hour,
			func() {
				blockHosts(timersCtx, "127.0.0.1", "youtube.com", "www.youtube.com")
			},
			func() {
				unblockHosts(timersCtx, "youtube.com", "www.youtube.com")
			})
		c.Edit("Timer for 1 hour was set")
		return c.Respond()
	})

	b.Handle(&bBlock, func(c tele.Context) error {
		fmt.Println("Youtube block request received")
		if timersCancelFunc != nil {
			timersCancelFunc()
		}
		blockHosts(timersCtx, "127.0.0.1", "youtube.com", "www.youtube.com")
		c.Edit("Youtube blocked")
		return c.Respond()
	})

	b.Handle(&bUnblock, func(c tele.Context) error {
		fmt.Println("Youtube unblock request received")
		if timersCancelFunc != nil {
			timersCancelFunc()
		}
		unblockHosts(timersCtx, "youtube.com", "www.youtube.com")
		c.Edit("Youtube unblocked")
		return c.Respond()
	})

	b.Start()
}

func startYoutubeTimer(c tele.Context, timersCtx context.Context, duration time.Duration, blocker func(), unblocker func()) {
	unblocker()
	defer blocker()

	fmt.Printf("Starting timer for %s\n", duration.String())
	select {
	case <-timersCtx.Done():
		fmt.Printf("Cancelling timer for %s\n", duration.String())
	case <-time.Tick(duration):
		c.Reply(fmt.Sprintf("%s timer finished...", duration.String()))
	}
}

func blockHosts(timersCtx context.Context, ip string, hosts ...string) {
	var hostManager = timersCtx.Value(types.HostsKey{}).(*txeh.Hosts)
	hostManager.AddHosts(ip, hosts)
	hfData := hostManager.RenderHostsFile()
	fmt.Printf("Blocked: %s\n", hfData)
	err := hostManager.Save()
	if err != nil {
		fmt.Printf("Failed to update /etc/hosts: %s\n", err.Error())
	}
}

func unblockHosts(timersCtx context.Context, hosts ...string) {
	var hostManager = timersCtx.Value(types.HostsKey{}).(*txeh.Hosts)
	hostManager.RemoveHosts(hosts)
	hfData := hostManager.RenderHostsFile()
	fmt.Printf("Unblocked: %s\n", hfData)
	err := hostManager.Save()
	if err != nil {
		fmt.Printf("Failed to update /etc/hosts: %s\n", err.Error())
	}
}

func makeStatisticsKeyboard(prev string, next string) *tele.ReplyMarkup {
	btns := []tele.Btn{}
	keyboard := &tele.ReplyMarkup{}
	if len(prev) > 0 {
		btns = append(btns, keyboard.Data("< "+prev, "prev"))
	}
	if len(next) > 0 {
		btns = append(btns, keyboard.Data(next+" >", "prev"))
	}

	keyboard.Inline(
		keyboard.Row(btns...),
	)

	return keyboard
}
