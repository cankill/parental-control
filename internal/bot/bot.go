package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"parental-control/internal/helper"
	"parental-control/internal/lib/config"
	"parental-control/internal/lib/types"
	"path/filepath"
	"sync"
	"time"

	tele "gopkg.in/telebot.v4"
	"gopkg.in/telebot.v4/middleware"
)

// youtubeDomains — домены, которые блокируются/разблокируются как единое целое.
var youtubeDomains = []string{"youtube.com", "www.youtube.com"}

var (
	selector   = &tele.ReplyMarkup{}
	b30Minutes = selector.Data("30 Minutes", "30-minutes")
	b1Hour     = selector.Data("1 Hour", "1-hour")
	bBlock     = selector.Data("Block", "block")
	bUnblock   = selector.Data("Unblock", "un-block")
	// defaultAdmins — фолбэк, если TG_ADMIN_IDS не задан в окружении.
	defaultAdmins = []int64{183358896}
)

// screenshotDir — каталог для временных снимков экрана (/screen).
const screenshotDir = "/tmp/pc"

// youtubeTimer инкапсулирует контекст активного таймера блокировки и сериализует
// обращения к helper'у. Telebot обрабатывает апдейты конкурентно, поэтому доступ
// к контексту таймера и к блокировке идёт под одним мьютексом.
type youtubeTimer struct {
	mu         sync.Mutex
	parent     context.Context
	ctx        context.Context
	cancelFunc context.CancelFunc
	client     *helper.Client
}

func newYoutubeTimer(parent context.Context) *youtubeTimer {
	ctx, cancel := context.WithCancel(parent)
	return &youtubeTimer{parent: parent, ctx: ctx, cancelFunc: cancel, client: helper.NewClient()}
}

// reset отменяет текущий таймер и заводит новый контекст, возвращая его.
func (t *youtubeTimer) reset() context.Context {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cancelFunc != nil {
		t.cancelFunc()
	}
	t.ctx, t.cancelFunc = context.WithCancel(t.parent)
	return t.ctx
}

// cancel отменяет текущий таймер без создания нового.
func (t *youtubeTimer) cancel() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cancelFunc != nil {
		t.cancelFunc()
	}
}

// block/unblock делегируют изменение /etc/hosts privileged helper'у (от root)
// и сериализуют вызовы под тем же мьютексом, что и контекст таймера.
func (t *youtubeTimer) block() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.client.BlockDomains(youtubeDomains); err != nil {
		fmt.Printf("Failed to block youtube via helper: %s\n", err)
	}
}

func (t *youtubeTimer) unblock() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.client.UnblockDomains(youtubeDomains); err != nil {
		fmt.Printf("Failed to unblock youtube via helper: %s\n", err)
	}
}

func StartBot(ctx context.Context, requests chan<- types.AppCommand) {
	fmt.Println("Running bot")
	env := ctx.Value(types.EnvKey{}).(*config.Env)
	timer := newYoutubeTimer(ctx)

	admins := env.AdminIDs
	if len(admins) == 0 {
		admins = defaultAdmins
	}

	pref := tele.Settings{
		Token:  env.BotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		// Не убиваем процесс через log.Fatal: даём горутине завершиться штатно,
		// чтобы статистика и остальные обработчики корректно остановились.
		log.Printf("Failed to create bot: %s", err)
		wg := ctx.Value(types.WgKey{}).(*sync.WaitGroup)
		wg.Done()
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
		// Буфер 1 + select с ctx.Done(): не блокируемся навсегда, если получатель
		// статистики уже завершился при шатдауне.
		responseChan := make(chan *types.AppInfoResponse, 1)
		select {
		case requests <- types.RequestCommand{ResponseChan: responseChan, ShiftHours: 0}:
		case <-ctx.Done():
			return c.Send("Statistics unavailable (shutting down)")
		}

		var statisticsResponse *types.AppInfoResponse
		select {
		case statisticsResponse = <-responseChan:
		case <-ctx.Done():
			return c.Send("Statistics unavailable (shutting down)")
		}

		statisticsResponse.AppInfos.SortByDurationDesc()
		statisticsTable := "```\n" + "  For: " + statisticsResponse.TimeStamp + "\n\n" + statisticsResponse.AppInfos.FormatTable() + "\n```"
		keyboard := makeStatisticsKeyboard("aaa", "")
		return c.Send(statisticsTable, &tele.SendOptions{ParseMode: tele.ModeMarkdownV2, ReplyMarkup: keyboard})
	})

	b.Handle("/screen", func(c tele.Context) error {
		if err := os.MkdirAll(screenshotDir, 0700); err != nil {
			return c.Send(fmt.Sprintf("Error: %s", err))
		}
		fname := filepath.Join(screenshotDir, fmt.Sprintf("capture-%d.jpg", time.Now().UnixNano()))
		cmd := exec.Command("/usr/sbin/screencapture", "-t", "jpg", "-x", fname)
		if err := cmd.Run(); err != nil {
			fmt.Println("Error: ", err)
			return c.Send(fmt.Sprintf("Error : %s", err))
		}
		defer os.Remove(fname)

		image := &tele.Photo{File: tele.FromDisk(fname)}
		return c.Send(image)
	})

	b.Handle("/youtube", func(c tele.Context) error {
		return c.Reply("For how long?", selector)
	})

	b.Handle(&b30Minutes, func(c tele.Context) error {
		fmt.Println("Youtube open for 30 minutes request received")
		timerCtx := timer.reset()
		go startYoutubeTimer(c, timerCtx, 30*time.Minute, timer.block, timer.unblock)
		c.Edit("Timer for 30 minutes was set")
		return c.Respond()
	})

	b.Handle(&b1Hour, func(c tele.Context) error {
		fmt.Println("Youtube open for 1 hour request received")
		timerCtx := timer.reset()
		go startYoutubeTimer(c, timerCtx, 1*time.Hour, timer.block, timer.unblock)
		c.Edit("Timer for 1 hour was set")
		return c.Respond()
	})

	b.Handle(&bBlock, func(c tele.Context) error {
		fmt.Println("Youtube block request received")
		timer.cancel()
		timer.block()
		c.Edit("Youtube blocked")
		return c.Respond()
	})

	b.Handle(&bUnblock, func(c tele.Context) error {
		fmt.Println("Youtube unblock request received")
		timer.cancel()
		timer.unblock()
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
	case <-time.After(duration):
		c.Reply(fmt.Sprintf("%s timer finished...", duration.String()))
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
