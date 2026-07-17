package bot

import (
	"parental-control/internal/browser"
	"strconv"
	"strings"

	tele "gopkg.in/telebot.v4"
)

func (h *handlerRegistry) registerStatsHandlers() {
	h.bot.Handle("/hourly", h.sendHourly)
	h.bot.Handle("/daily", h.sendDaily)
	h.bot.Handle("/info", h.sendAppInfo)
	h.bot.Handle("/stats", func(c tele.Context) error {
		return c.Send("Statistics:", h.keyboards.stats)
	})
	h.bot.Handle(&h.keyboards.hourly, h.hubAction("/hourly", h.sendHourly))
	h.bot.Handle(&h.keyboards.daily, h.hubAction("/daily", h.sendDaily))
	h.bot.Handle(&h.keyboards.statPrev, h.navigateHourly)
	h.bot.Handle(&h.keyboards.statNext, h.navigateHourly)
	h.bot.Handle(&h.keyboards.dayPrev, h.navigateDaily)
	h.bot.Handle(&h.keyboards.dayNext, h.navigateDaily)
}

func (h *handlerRegistry) sendHourly(c tele.Context) error {
	resp, err := h.stats.hourly(0)
	if err != nil {
		return c.Send("Statistics unavailable (shutting down)")
	}
	text, kb := renderStatistics(resp)
	if err := c.Send(text, markdown(kb)); err != nil {
		return err
	}

	url, _ := browser.FrontmostBrowserURL()
	if url == "" {
		return nil
	}
	sites, err := h.stats.sites(0)
	if err != nil {
		return nil
	}
	sites.ActiveApp = browser.Domain(url)
	sitesText, sitesKeyboard := renderSites(sites)
	return c.Send(sitesText, markdown(sitesKeyboard))
}

func (h *handlerRegistry) navigateHourly(c tele.Context) error {
	shift := callbackShift(c)
	resp, err := h.stats.hourly(shift)
	if err == nil {
		text, kb := renderStatistics(resp)
		_ = c.Edit(text, markdown(kb))
	}
	return c.Respond()
}

func (h *handlerRegistry) sendDaily(c tele.Context) error {
	resp, err := h.stats.daily(0)
	if err != nil {
		return c.Send("Statistics unavailable (shutting down)")
	}
	text, kb := renderDaily(resp)
	return c.Send(text, markdown(kb))
}

func (h *handlerRegistry) navigateDaily(c tele.Context) error {
	resp, err := h.stats.daily(callbackShift(c))
	if err == nil {
		text, kb := renderDaily(resp)
		_ = c.Edit(text, markdown(kb))
	}
	return c.Respond()
}

func (h *handlerRegistry) sendAppInfo(c tele.Context) error {
	name := strings.TrimSpace(c.Message().Payload)
	if name == "" {
		return c.Send("Usage: /info <app name from /status>")
	}
	text, err := h.stats.appInfo(name)
	if err != nil {
		return c.Send("Unavailable (shutting down)")
	}
	return c.Send("```\n"+text+"\n```", &tele.SendOptions{ParseMode: tele.ModeMarkdownV2})
}

func callbackShift(c tele.Context) int {
	shift, _ := strconv.Atoi(c.Data())
	if shift < 0 {
		return 0
	}
	return shift
}

func markdown(kb *tele.ReplyMarkup) *tele.SendOptions {
	return &tele.SendOptions{ParseMode: tele.ModeMarkdownV2, ReplyMarkup: kb}
}
