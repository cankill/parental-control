package bot

import (
	"os"
	"parental-control/internal/media"
	"strconv"
	"strings"
)

func (h *handlerRegistry) registerMediaHandlers() {
	h.command("screen", h.sendScreen)
	h.command("video", h.sendVideo)
	h.command("photo", h.sendVideo) // Compatibility with old commands and messages.
	h.command("record", h.sendRecord)
	h.command("media", func(c *updateContext) error {
		return c.SendRichMessage(renderMenu("Media",
			richCallbackButton("Video", "hub-video"),
			richCallbackButton("Screen", "hub-screen"),
			richCallbackButton("Audio", "hub-record"),
		))
	})
	h.callback("hub-video", h.hubAction(h.sendVideo))
	h.callback("hub-photo", h.hubAction(h.sendVideo)) // Keep old inline buttons working.
	h.callback("hub-screen", h.hubAction(h.sendScreen))
	h.callback("hub-record", h.hubAction(h.sendRecord))
}

func (h *handlerRegistry) sendScreen(c *updateContext) error {
	fname, err := media.CaptureScreen()
	if err != nil {
		return c.RespondRichMessage(renderNotice("Screenshot failed", err.Error()))
	}
	defer os.Remove(fname)
	file, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer file.Close()
	return c.RespondRichMessage(renderPhoto(file, "screen.png", "Screenshot"))
}

func (h *handlerRegistry) sendVideo(c *updateContext) error {
	fname, err := media.CaptureVideo()
	if err != nil {
		return c.RespondRichMessage(renderNotice("Video failed", err.Error()))
	}
	defer os.Remove(fname)
	file, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer file.Close()
	return c.RespondRichMessage(renderVideo(file, "camera.mp4", "Camera · 5 seconds"))
}

func (h *handlerRegistry) sendRecord(c *updateContext) error {
	fname, err := media.RecordAudio(recordSeconds(c))
	if err != nil {
		return c.RespondRichMessage(renderNotice("Audio recording failed", err.Error()))
	}
	defer os.Remove(fname)
	file, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer file.Close()
	return c.RespondRichMessage(renderAudio(file, "recording.m4a", "Audio recording"))
}

func recordSeconds(c *updateContext) int {
	if payload := strings.TrimSpace(c.Payload()); payload != "" {
		if seconds, err := strconv.Atoi(payload); err == nil {
			return seconds
		}
	}
	return 5
}
