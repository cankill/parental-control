package bot

import (
	"fmt"
	"os"
	"parental-control/internal/media"
	"strconv"
	"strings"

	tele "gopkg.in/telebot.v4"
)

func (h *handlerRegistry) registerMediaHandlers() {
	h.bot.Handle("/screen", h.sendScreen)
	h.bot.Handle("/photo", h.sendPhoto)
	h.bot.Handle("/record", h.sendRecord)
	h.bot.Handle("/media", func(c tele.Context) error {
		return c.Send("Media:", h.keyboards.media)
	})
	h.bot.Handle(&h.keyboards.photo, h.hubAction("/photo", h.sendPhoto))
	h.bot.Handle(&h.keyboards.screen, h.hubAction("/screen", h.sendScreen))
	h.bot.Handle(&h.keyboards.record, h.hubAction("/record", h.sendRecord))
}

func (h *handlerRegistry) sendScreen(c tele.Context) error {
	fname, err := media.CaptureScreen()
	if err != nil {
		return c.Send(err.Error())
	}
	defer os.Remove(fname)
	return c.Send(&tele.Photo{File: tele.FromDisk(fname)})
}

func (h *handlerRegistry) sendPhoto(c tele.Context) error {
	fname, err := media.CapturePhoto()
	if err != nil {
		return c.Send(fmt.Sprintf("Photo error: %s", err))
	}
	defer os.Remove(fname)
	return c.Send(&tele.Photo{File: tele.FromDisk(fname)})
}

func (h *handlerRegistry) sendRecord(c tele.Context) error {
	fname, err := media.RecordAudio(recordSeconds(c))
	if err != nil {
		return c.Send(fmt.Sprintf("Record error: %s", err))
	}
	defer os.Remove(fname)
	return c.Send(&tele.Audio{File: tele.FromDisk(fname)})
}

func recordSeconds(c tele.Context) int {
	if payload := strings.TrimSpace(c.Message().Payload); payload != "" {
		if seconds, err := strconv.Atoi(payload); err == nil {
			return seconds
		}
	}
	return 5
}
