package bot

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"parental-control/internal/browser"
	"parental-control/internal/helper"
	"parental-control/internal/lib/config"
	"parental-control/internal/lib/types"
	"parental-control/internal/media"
	"path/filepath"
	"strconv"
	"strings"
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

	// Кнопки навигации по часам под /status. Unique задаёт callback-роут, а
	// конкретный целевой shiftHours передаётся в payload (Data) при отрисовке.
	statNav     = &tele.ReplyMarkup{}
	btnStatPrev = statNav.Data("‹ Earlier", "stat-prev")
	btnStatNext = statNav.Data("Later ›", "stat-next")

	// Кнопки навигации по дням под /daily (payload = целевой dayShift).
	dayNav     = &tele.ReplyMarkup{}
	btnDayPrev = dayNav.Data("‹ Prev day", "day-prev")
	btnDayNext = dayNav.Data("Next day ›", "day-next")

	// Кнопки навигации по часам под /sites (статистика доменов; payload = shiftHours).
	sitesNav     = &tele.ReplyMarkup{}
	btnSitesPrev = sitesNav.Data("‹ Earlier", "sites-prev")
	btnSitesNext = sitesNav.Data("Later ›", "sites-next")

	// Хаб-клавиатуры: команда-хаб (/stats, /media, /web) показывает кнопки, каждая
	// из которых делает то же, что соответствующая прямая команда.
	statsHub    = &tele.ReplyMarkup{}
	btnHourly   = statsHub.Data("Hourly", "hub-hourly")
	btnDaily    = statsHub.Data("Daily", "hub-daily")
	mediaHub    = &tele.ReplyMarkup{}
	btnPhoto    = mediaHub.Data("Photo", "hub-photo")
	btnScreen   = mediaHub.Data("Screen", "hub-screen")
	btnRecord   = mediaHub.Data("Record", "hub-record")
	webHub      = &tele.ReplyMarkup{}
	btnWebURL   = webHub.Data("URL", "hub-url")
	btnWebSites = webHub.Data("Sites", "hub-sites")
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
	statsHub.Inline(statsHub.Row(btnHourly, btnDaily))
	mediaHub.Inline(mediaHub.Row(btnPhoto, btnScreen, btnRecord))
	webHub.Inline(webHub.Row(btnWebURL, btnWebSites))

	// Регистрируем список команд — Telegram покажет их в меню «/» рядом с полем
	// ввода. Ошибку только логируем: без списка бот всё равно работает.
	if err := b.SetCommands([]tele.Command{
		{Text: "stats", Description: "App usage: Hourly | Daily"},
		{Text: "hourly", Description: "App usage this hour"},
		{Text: "daily", Description: "App usage today"},
		{Text: "info", Description: "App info by name: /info <name>"},
		{Text: "web", Description: "Browser: URL | Sites"},
		{Text: "url", Description: "Current browser URL"},
		{Text: "sites", Description: "Domain usage this hour"},
		{Text: "media", Description: "Capture: Photo | Screen | Record"},
		{Text: "photo", Description: "Photo from camera"},
		{Text: "screen", Description: "Screenshot"},
		{Text: "record", Description: "Record audio: /record [seconds]"},
		{Text: "youtube", Description: "Block / unblock YouTube"},
	}); err != nil {
		log.Printf("SetCommands failed: %s", err)
	}

	go func() {
		<-ctx.Done()
		b.Stop()
		fmt.Println("Bot stopped")
		wg := ctx.Value(types.WgKey{}).(*sync.WaitGroup)
		wg.Done()
	}()

	b.Use(middleware.Whitelist(admins...))
	// b.Use(middleware.Logger())

	// fetchStatistics запрашивает у горутины статистики срез за shiftHours назад.
	fetchStatistics := func(shiftHours int) (*types.AppInfoResponse, error) {
		responseChan := make(chan *types.AppInfoResponse, 1)
		select {
		case requests <- types.RequestCommand{ResponseChan: responseChan, ShiftHours: shiftHours}:
		case <-ctx.Done():
			return nil, fmt.Errorf("shutting down")
		}
		select {
		case resp := <-responseChan:
			return resp, nil
		case <-ctx.Done():
			return nil, fmt.Errorf("shutting down")
		}
	}

	// sendHourly отправляет статистику приложений за текущий час (переиспользуется
	// прямой командой /hourly и кнопкой Hourly в хабе /stats).
	sendHourly := func(c tele.Context) error {
		resp, err := fetchStatistics(0)
		if err != nil {
			return c.Send("Statistics unavailable (shutting down)")
		}
		text, kb := renderStatistics(resp)
		return c.Send(text, &tele.SendOptions{ParseMode: tele.ModeMarkdownV2, ReplyMarkup: kb})
	}
	b.Handle("/hourly", sendHourly)

	// Навигация по часам: prev = глубже в прошлое (shift+1), next = ближе к сейчас
	// (shift-1). Целевой shift берётся из payload callback'а. Сообщение
	// перерисовывается на месте через c.Edit.
	navHandler := func(c tele.Context) error {
		shift, _ := strconv.Atoi(c.Data())
		if shift < 0 {
			shift = 0
		}
		resp, err := fetchStatistics(shift)
		if err != nil {
			return c.Respond()
		}
		text, kb := renderStatistics(resp)
		_ = c.Edit(text, &tele.SendOptions{ParseMode: tele.ModeMarkdownV2, ReplyMarkup: kb})
		return c.Respond()
	}
	b.Handle(&btnStatPrev, navHandler)
	b.Handle(&btnStatNext, navHandler)

	// fetchDay запрашивает агрегированную статистику за календарный день dayShift.
	fetchDay := func(dayShift int) (*types.AppInfoResponse, error) {
		responseChan := make(chan *types.AppInfoResponse, 1)
		select {
		case requests <- types.DayRequest{DayShift: dayShift, ResponseChan: responseChan}:
		case <-ctx.Done():
			return nil, fmt.Errorf("shutting down")
		}
		select {
		case resp := <-responseChan:
			return resp, nil
		case <-ctx.Done():
			return nil, fmt.Errorf("shutting down")
		}
	}

	// sendDaily отправляет статистику за календарный день (сегодня), с навигацией по
	// дням (переиспользуется командой /daily и кнопкой Daily в хабе /stats).
	sendDaily := func(c tele.Context) error {
		resp, err := fetchDay(0)
		if err != nil {
			return c.Send("Statistics unavailable (shutting down)")
		}
		text, kb := renderDaily(resp)
		return c.Send(text, &tele.SendOptions{ParseMode: tele.ModeMarkdownV2, ReplyMarkup: kb})
	}
	b.Handle("/daily", sendDaily)

	// /stats — хаб: клавиатура выбора Hourly | Daily. Кнопки делают то же, что
	// прямые /hourly и /daily. Callback от кнопки шлёт новое сообщение (не Edit),
	// т.к. это переход к полноценному отчёту со своей навигацией.
	b.Handle("/stats", func(c tele.Context) error {
		return c.Send("Statistics:", statsHub)
	})
	b.Handle(&btnHourly, func(c tele.Context) error { _ = c.Respond(); return sendHourly(c) })
	b.Handle(&btnDaily, func(c tele.Context) error { _ = c.Respond(); return sendDaily(c) })

	dayNavHandler := func(c tele.Context) error {
		shift, _ := strconv.Atoi(c.Data())
		if shift < 0 {
			shift = 0
		}
		resp, err := fetchDay(shift)
		if err != nil {
			return c.Respond()
		}
		text, kb := renderDaily(resp)
		_ = c.Edit(text, &tele.SendOptions{ParseMode: tele.ModeMarkdownV2, ReplyMarkup: kb})
		return c.Respond()
	}
	b.Handle(&btnDayPrev, dayNavHandler)
	b.Handle(&btnDayNext, dayNavHandler)

	// /info <name> — метаданные приложения из словаря по отображаемому имени
	// (то, что видно в /status). Например: /info Client.
	b.Handle("/info", func(c tele.Context) error {
		name := strings.TrimSpace(c.Message().Payload)
		if name == "" {
			return c.Send("Usage: /info <app name from /status>")
		}
		responseChan := make(chan string, 1)
		select {
		case requests <- types.AppInfoQuery{Name: name, ResponseChan: responseChan}:
		case <-ctx.Done():
			return c.Send("Unavailable (shutting down)")
		}
		select {
		case text := <-responseChan:
			return c.Send("```\n" + text + "\n```", &tele.SendOptions{ParseMode: tele.ModeMarkdownV2})
		case <-ctx.Done():
			return c.Send("Unavailable (shutting down)")
		}
	})

	// sendURL — текущий URL активной вкладки frontmost-браузера (мгновенно).
	sendURL := func(c tele.Context) error {
		url, err := browser.FrontmostBrowserURL()
		if err != nil {
			return c.Send(fmt.Sprintf("No browser URL: %s", err))
		}
		if url == "" {
			return c.Send("No active browser tab")
		}
		return c.Send(url)
	}
	b.Handle("/url", sendURL)

	// fetchSites запрашивает статистику доменов за час shiftHours.
	fetchSites := func(shiftHours int) (*types.AppInfoResponse, error) {
		responseChan := make(chan *types.AppInfoResponse, 1)
		select {
		case requests <- types.DomainRequest{ShiftHours: shiftHours, ResponseChan: responseChan}:
		case <-ctx.Done():
			return nil, fmt.Errorf("shutting down")
		}
		select {
		case resp := <-responseChan:
			return resp, nil
		case <-ctx.Done():
			return nil, fmt.Errorf("shutting down")
		}
	}

	// sendSites — статистика по доменам за текущий час, с навигацией по часам.
	sendSites := func(c tele.Context) error {
		resp, err := fetchSites(0)
		if err != nil {
			return c.Send("Statistics unavailable (shutting down)")
		}
		text, kb := renderSites(resp)
		return c.Send(text, &tele.SendOptions{ParseMode: tele.ModeMarkdownV2, ReplyMarkup: kb})
	}
	b.Handle("/sites", sendSites)

	// /web — хаб: URL | Sites.
	b.Handle("/web", func(c tele.Context) error {
		return c.Send("Web:", webHub)
	})
	b.Handle(&btnWebURL, func(c tele.Context) error { _ = c.Respond(); return sendURL(c) })
	b.Handle(&btnWebSites, func(c tele.Context) error { _ = c.Respond(); return sendSites(c) })

	sitesNavHandler := func(c tele.Context) error {
		shift, _ := strconv.Atoi(c.Data())
		if shift < 0 {
			shift = 0
		}
		resp, err := fetchSites(shift)
		if err != nil {
			return c.Respond()
		}
		text, kb := renderSites(resp)
		_ = c.Edit(text, &tele.SendOptions{ParseMode: tele.ModeMarkdownV2, ReplyMarkup: kb})
		return c.Respond()
	}
	b.Handle(&btnSitesPrev, sitesNavHandler)
	b.Handle(&btnSitesNext, sitesNavHandler)

	// sendScreen — снимок экрана.
	sendScreen := func(c tele.Context) error {
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
		return c.Send(&tele.Photo{File: tele.FromDisk(fname)})
	}
	b.Handle("/screen", sendScreen)

	// sendPhoto — снимок с камеры. Требует TCC Camera (диалог один раз). macOS
	// зажигает индикатор камеры на время съёмки — это неотключаемо.
	sendPhoto := func(c tele.Context) error {
		fname, err := media.CapturePhoto()
		if err != nil {
			return c.Send(fmt.Sprintf("Photo error: %s", err))
		}
		defer os.Remove(fname)
		return c.Send(&tele.Photo{File: tele.FromDisk(fname)})
	}
	b.Handle("/photo", sendPhoto)

	// /record [N] — запись N секунд аудио с микрофона (дефолт 5, максимум 60).
	// Требует TCC Microphone (диалог один раз); индикатор микрофона горит при записи.
	// recordSeconds берёт N из payload (только у прямой команды; кнопка → дефолт).
	recordSeconds := func(c tele.Context) int {
		if p := strings.TrimSpace(c.Message().Payload); p != "" {
			if n, err := strconv.Atoi(p); err == nil {
				return n
			}
		}
		return 5
	}
	sendRecord := func(c tele.Context) error {
		fname, err := media.RecordAudio(recordSeconds(c))
		if err != nil {
			return c.Send(fmt.Sprintf("Record error: %s", err))
		}
		defer os.Remove(fname)
		return c.Send(&tele.Audio{File: tele.FromDisk(fname)})
	}
	b.Handle("/record", sendRecord)

	// /media — хаб: Photo | Screen | Record (кнопка Record пишет дефолтные 5с;
	// для другой длительности — прямая команда /record N).
	b.Handle("/media", func(c tele.Context) error {
		return c.Send("Media:", mediaHub)
	})
	b.Handle(&btnPhoto, func(c tele.Context) error { _ = c.Respond(); return sendPhoto(c) })
	b.Handle(&btnScreen, func(c tele.Context) error { _ = c.Respond(); return sendScreen(c) })
	b.Handle(&btnRecord, func(c tele.Context) error { _ = c.Respond(); return sendRecord(c) })

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

// renderStatistics формирует текст таблицы статистики и клавиатуру навигации
// для конкретного среза (resp несёт TimeStamp, ShiftHours и флаги HasOlder/HasNewer).
func renderStatistics(resp *types.AppInfoResponse) (string, *tele.ReplyMarkup) {
	resp.AppInfos.SortByDurationDesc()
	// ● помечает активное приложение (только в текущем часе — ActiveApp пуст иначе).
	text := "```\n" + "  For: " + resp.TimeStamp + "\n\n" + resp.AppInfos.FormatTableMarked(resp.ActiveApp) + "\n```"
	return text, makeStatisticsKeyboard(resp)
}

// makeStatisticsKeyboard строит ряд навигации. Стрелка показывается только если в
// ту сторону есть данные, а её payload — целевой shift ближайшего непустого часа
// (пропущенные часы уже перепрыгнуты в NearestShift). Хендлер не хранит состояние.
func makeStatisticsKeyboard(resp *types.AppInfoResponse) *tele.ReplyMarkup {
	kb := &tele.ReplyMarkup{}
	btns := []tele.Btn{}
	if resp.HasOlder {
		btns = append(btns, kb.Data("‹ Earlier", "stat-prev", strconv.Itoa(resp.OlderShift)))
	}
	if resp.HasNewer {
		btns = append(btns, kb.Data("Later ›", "stat-next", strconv.Itoa(resp.NewerShift)))
	}
	if len(btns) == 0 {
		return kb
	}
	kb.Inline(kb.Row(btns...))
	return kb
}

// renderDaily формирует таблицу за календарный день и клавиатуру навигации по дням.
func renderDaily(resp *types.AppInfoResponse) (string, *tele.ReplyMarkup) {
	resp.AppInfos.SortByDurationDesc()
	text := "```\n" + "  Day: " + resp.TimeStamp + "\n\n" + resp.AppInfos.FormatTable() + "\n```"
	return text, makeDayKeyboard(resp)
}

// makeDayKeyboard — навигация по дням: стрелка ведёт к ближайшему непустому дню
// (пустые перепрыгнуты в NearestDayShift), payload несёт целевой dayShift.
func makeDayKeyboard(resp *types.AppInfoResponse) *tele.ReplyMarkup {
	kb := &tele.ReplyMarkup{}
	btns := []tele.Btn{}
	if resp.HasOlder {
		btns = append(btns, kb.Data("‹ Prev day", "day-prev", strconv.Itoa(resp.OlderShift)))
	}
	if resp.HasNewer {
		btns = append(btns, kb.Data("Next day ›", "day-next", strconv.Itoa(resp.NewerShift)))
	}
	if len(btns) == 0 {
		return kb
	}
	kb.Inline(kb.Row(btns...))
	return kb
}

// renderSites формирует таблицу статистики доменов за час и навигацию по часам.
func renderSites(resp *types.AppInfoResponse) (string, *tele.ReplyMarkup) {
	resp.AppInfos.SortByDurationDesc()
	text := "```\n" + "  Sites for: " + resp.TimeStamp + "\n\n" + resp.AppInfos.FormatTable() + "\n```"
	kb := &tele.ReplyMarkup{}
	btns := []tele.Btn{}
	if resp.HasOlder {
		btns = append(btns, kb.Data("‹ Earlier", "sites-prev", strconv.Itoa(resp.OlderShift)))
	}
	if resp.HasNewer {
		btns = append(btns, kb.Data("Later ›", "sites-next", strconv.Itoa(resp.NewerShift)))
	}
	if len(btns) > 0 {
		kb.Inline(kb.Row(btns...))
	}
	return text, kb
}
