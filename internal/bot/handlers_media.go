package bot

import (
	"fmt"
	"os"
	"parental-control/internal/media"
	"strconv"
	"strings"
)

func (h *handlerRegistry) registerMediaHandlers() {
	h.command("screen", h.sendScreen)
	h.command("photo", h.sendPhoto)
	h.command("record", h.sendRecord)
	h.command("media", func(c *updateContext) error {
		return c.SendText("Media:", "", h.keyboards.media)
	})
	h.callback("hub-photo", h.hubAction("/photo", h.sendPhoto))
	h.callback("hub-screen", h.hubAction("/screen", h.sendScreen))
	h.callback("hub-record", h.hubAction("/record", h.sendRecord))
}

func (h *handlerRegistry) sendScreen(c *updateContext) error {
	fname, err := media.CaptureScreen()
	if err != nil {
		return c.SendText(err.Error(), "", nil)
	}
	defer os.Remove(fname)
	return c.SendPhotoFile(fname)
}

func (h *handlerRegistry) sendPhoto(c *updateContext) error {
	fname, err := media.CapturePhoto()
	if err != nil {
		return c.SendText(fmt.Sprintf("Photo error: %s", err), "", nil)
	}
	defer os.Remove(fname)
	return c.SendPhotoFile(fname)
}

func (h *handlerRegistry) sendRecord(c *updateContext) error {
	fname, err := media.RecordAudio(recordSeconds(c))
	if err != nil {
		return c.SendText(fmt.Sprintf("Record error: %s", err), "", nil)
	}
	defer os.Remove(fname)
	return c.SendAudioFile(fname)
}

func recordSeconds(c *updateContext) int {
	if payload := strings.TrimSpace(c.Payload()); payload != "" {
		if seconds, err := strconv.Atoi(payload); err == nil {
			return seconds
		}
	}
	return 5
}
