package bot

import (
	"context"
	"fmt"
	"time"

	tele "gopkg.in/telebot.v4"
)

func (h *handlerRegistry) registerYoutubeHandlers() {
	h.bot.Handle("/youtube", func(c tele.Context) error {
		return c.Reply("For how long?", h.keyboards.youtube)
	})
	h.bot.Handle(&h.keyboards.minutes30, h.youtubeDuration(30*time.Minute, "30 minutes"))
	h.bot.Handle(&h.keyboards.hour1, h.youtubeDuration(time.Hour, "1 hour"))
	h.bot.Handle(&h.keyboards.block, func(c tele.Context) error {
		h.youtube.cancel()
		h.youtube.block()
		_ = c.Edit("Youtube blocked")
		return c.Respond()
	})
	h.bot.Handle(&h.keyboards.unblock, func(c tele.Context) error {
		h.youtube.cancel()
		h.youtube.unblock()
		_ = c.Edit("Youtube unblocked")
		return c.Respond()
	})
}

func (h *handlerRegistry) youtubeDuration(duration time.Duration, label string) tele.HandlerFunc {
	return func(c tele.Context) error {
		timerCtx := h.youtube.reset()
		go startYoutubeTimer(c, timerCtx, duration, h.youtube.block, h.youtube.unblock)
		_ = c.Edit(fmt.Sprintf("Timer for %s was set", label))
		return c.Respond()
	}
}

func startYoutubeTimer(c tele.Context, timerCtx context.Context, duration time.Duration, blocker, unblocker func()) {
	unblocker()
	fmt.Printf("Starting timer for %s\n", duration)
	select {
	case <-timerCtx.Done():
		fmt.Printf("Cancelling timer for %s\n", duration)
	case <-time.After(duration):
		blocker()
		_, _ = c.Bot().Send(c.Recipient(), fmt.Sprintf("%s timer finished...", duration))
	}
}
