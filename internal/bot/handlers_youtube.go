package bot

import (
	"context"
	"fmt"
	"time"

	tgbot "github.com/go-telegram/bot"
)

func (h *handlerRegistry) registerYoutubeHandlers() {
	h.command("youtube", func(c *updateContext) error {
		return c.ReplyText("For how long?", h.keyboards.youtube)
	})
	h.callback("30-minutes", h.youtubeDuration(30*time.Minute, "30 minutes"))
	h.callback("1-hour", h.youtubeDuration(time.Hour, "1 hour"))
	h.callback("block", func(c *updateContext) error {
		h.youtube.cancel()
		h.youtube.block()
		return c.EditText("Youtube blocked", "", nil)
	})
	h.callback("un-block", func(c *updateContext) error {
		h.youtube.cancel()
		h.youtube.unblock()
		return c.EditText("Youtube unblocked", "", nil)
	})
}

func (h *handlerRegistry) youtubeDuration(duration time.Duration, label string) handlerFunc {
	return func(c *updateContext) error {
		chatID, ok := c.chatID()
		if !ok {
			return fmt.Errorf("callback has no accessible chat")
		}
		timerCtx := h.youtube.reset()
		go startYoutubeTimer(timerCtx, c.bot, chatID, duration, h.youtube.block, h.youtube.unblock)
		return c.EditText(fmt.Sprintf("Timer for %s was set", label), "", nil)
	}
}

func startYoutubeTimer(ctx context.Context, b *tgbot.Bot, chatID int64, duration time.Duration, blocker, unblocker func()) {
	unblocker()
	fmt.Printf("Starting timer for %s\n", duration)
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		fmt.Printf("Cancelling timer for %s\n", duration)
	case <-timer.C:
		blocker()
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: fmt.Sprintf("%s timer finished...", duration)})
	}
}
