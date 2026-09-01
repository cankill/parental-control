package bot

import (
	"parental-control/internal/browser"
	"strconv"
	"strings"

	"github.com/go-telegram/bot/models"
)

func (h *handlerRegistry) registerStatsHandlers() {
	h.command("hourly", h.sendHourly)
	h.command("daily", h.sendDaily)
	h.command("info", h.sendAppInfo)
	h.command("stats", func(c *updateContext) error {
		return c.SendText("Statistics:", "", h.keyboards.stats)
	})
	h.callback("hub-hourly", h.hubAction("/hourly", h.sendHourly))
	h.callback("hub-daily", h.hubAction("/daily", h.sendDaily))
	h.callback("stat-prev", h.navigateHourly)
	h.callback("stat-next", h.navigateHourly)
	h.callback("day-prev", h.navigateDaily)
	h.callback("day-next", h.navigateDaily)
}

func (h *handlerRegistry) sendHourly(c *updateContext) error {
	resp, err := h.stats.hourly(0)
	if err != nil {
		return c.SendText("Statistics unavailable (shutting down)", "", nil)
	}
	text, kb := renderStatistics(resp)
	if err := c.SendText(text, models.ParseModeMarkdown, kb); err != nil {
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
	return c.SendText(sitesText, models.ParseModeMarkdown, sitesKeyboard)
}

func (h *handlerRegistry) navigateHourly(c *updateContext) error {
	resp, err := h.stats.hourly(callbackShift(c))
	if err != nil {
		return err
	}
	text, kb := renderStatistics(resp)
	return c.EditText(text, models.ParseModeMarkdown, kb)
}

func (h *handlerRegistry) sendDaily(c *updateContext) error {
	resp, err := h.stats.daily(0)
	if err != nil {
		return c.SendText("Statistics unavailable (shutting down)", "", nil)
	}
	return c.SendRichMessage(renderDailyRich(resp))
}

func (h *handlerRegistry) navigateDaily(c *updateContext) error {
	resp, err := h.stats.daily(callbackShift(c))
	if err != nil {
		return err
	}
	return c.EditRichMessage(renderDailyRich(resp))
}

func (h *handlerRegistry) sendAppInfo(c *updateContext) error {
	name := strings.TrimSpace(c.Payload())
	if name == "" {
		return c.SendText("Usage: /info <app name from /status>", "", nil)
	}
	text, err := h.stats.appInfo(name)
	if err != nil {
		return c.SendText("Unavailable (shutting down)", "", nil)
	}
	return c.SendText("```\n"+text+"\n```", models.ParseModeMarkdown, nil)
}

func callbackShift(c *updateContext) int {
	shift, _ := strconv.Atoi(c.Data())
	if shift < 0 {
		return 0
	}
	return shift
}
