package bot

import (
	"context"
	"fmt"
	"time"
)

func (h *handlerRegistry) registerYoutubeHandlers() {
	h.command("youtube", func(c *updateContext) error {
		return c.ReplyRichMessage(renderMenu("YouTube access",
			richCallbackButton("30 minutes", "30-minutes"),
			richCallbackButton("1 hour", "1-hour"),
			richCallbackButton("Block", "block"),
			richCallbackButton("Unblock", "un-block"),
		))
	})
	h.callback("30-minutes", h.youtubeDuration(30*time.Minute, "30 minutes"))
	h.callback("1-hour", h.youtubeDuration(time.Hour, "1 hour"))
	h.callback("block", func(c *updateContext) error {
		h.youtube.cancel()
		h.youtube.block()
		return c.EditRichMessage(renderStatus("YouTube blocked."))
	})
	h.callback("un-block", func(c *updateContext) error {
		h.youtube.cancel()
		h.youtube.unblock()
		return c.EditRichMessage(renderStatus("YouTube unblocked."))
	})
}

func (h *handlerRegistry) youtubeDuration(duration time.Duration, label string) handlerFunc {
	return func(c *updateContext) error {
		chatID, ok := c.chatID()
		if !ok {
			return fmt.Errorf("callback has no accessible chat")
		}
		timerCtx := h.youtube.reset()
		go startYoutubeTimer(timerCtx, c.rich, chatID, duration, h.youtube.block, h.youtube.unblock)
		return c.EditRichMessage(renderStatus(fmt.Sprintf("YouTube will be blocked in %s.", label)))
	}
}

func startYoutubeTimer(ctx context.Context, rich *richClient, chatID int64, duration time.Duration, blocker, unblocker func()) {
	unblocker()
	fmt.Printf("Starting timer for %s\n", duration)
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		fmt.Printf("Cancelling timer for %s\n", duration)
	case <-timer.C:
		blocker()
		_ = rich.send(ctx, chatID, renderStatus(fmt.Sprintf("The %s YouTube timer has finished. YouTube is now blocked.", duration)), 0)
	}
}
