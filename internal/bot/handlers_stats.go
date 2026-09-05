package bot

import (
	"parental-control/internal/browser"
	"strconv"
	"strings"
)

func (h *handlerRegistry) registerStatsHandlers() {
	h.command("hourly", h.sendHourly)
	h.command("daily", h.sendDaily)
	h.command("weekly", h.sendWeekly)
	h.command("info", h.sendAppInfo)
	h.command("stats", func(c *updateContext) error {
		return c.SendRichMessage(renderStatsMenu())
	})
	h.callback("hub-hourly", h.hubAction(h.sendHourly))
	h.callback("hub-daily", h.hubAction(h.sendDaily))
	h.callback("hub-weekly", h.hubAction(h.sendWeekly))
	h.callback("stat-prev", h.navigateHourly)
	h.callback("stat-next", h.navigateHourly)
	h.callback("day-prev", h.navigateDaily)
	h.callback("day-next", h.navigateDaily)
	h.callback("week-prev", h.navigateWeekly)
	h.callback("week-next", h.navigateWeekly)
}

func (h *handlerRegistry) sendHourly(c *updateContext) error {
	resp, err := h.stats.hourly(0)
	if err != nil {
		return c.RespondRichMessage(renderNotice("Statistics unavailable", "ParentControl is shutting down."))
	}
	message := renderStatistics(resp)

	url, _ := browser.FrontmostBrowserURL()
	if url == "" {
		return c.RespondRichMessage(message)
	}
	sites, err := h.stats.sites(0)
	if err != nil {
		return c.RespondRichMessage(message)
	}
	sites.ActiveApp = browser.Domain(url)
	message.Blocks = append(message.Blocks, renderSites(sites).Blocks...)
	return c.RespondRichMessage(message)
}

func (h *handlerRegistry) navigateHourly(c *updateContext) error {
	resp, err := h.stats.hourly(callbackShift(c))
	if err != nil {
		return err
	}
	return c.EditRichMessage(renderStatistics(resp))
}

func (h *handlerRegistry) sendDaily(c *updateContext) error {
	resp, err := h.stats.daily(0)
	if err != nil {
		return c.RespondRichMessage(renderNotice("Statistics unavailable", "ParentControl is shutting down."))
	}
	return c.RespondRichMessage(renderDailyRich(resp))
}

func (h *handlerRegistry) navigateDaily(c *updateContext) error {
	resp, err := h.stats.daily(callbackShift(c))
	if err != nil {
		return err
	}
	return c.EditRichMessage(renderDailyRich(resp))
}

func (h *handlerRegistry) sendWeekly(c *updateContext) error {
	resp, err := h.stats.weekly(0)
	if err != nil {
		return c.RespondRichMessage(renderNotice("Statistics unavailable", "ParentControl is shutting down."))
	}
	return c.RespondRichMessage(renderWeeklyRich(resp))
}

func (h *handlerRegistry) navigateWeekly(c *updateContext) error {
	resp, err := h.stats.weekly(callbackShift(c))
	if err != nil {
		return err
	}
	return c.EditRichMessage(renderWeeklyRich(resp))
}

func (h *handlerRegistry) sendAppInfo(c *updateContext) error {
	name := strings.TrimSpace(c.Payload())
	if name == "" {
		return c.RespondRichMessage(renderNotice("Usage", "/info <app name from /stats>"))
	}
	text, err := h.stats.appInfo(name)
	if err != nil {
		return c.RespondRichMessage(renderNotice("Application info unavailable", "ParentControl is shutting down."))
	}
	return c.RespondRichMessage(renderPreformatted("Application info", text))
}

func callbackShift(c *updateContext) int {
	shift, _ := strconv.Atoi(c.Data())
	if shift < 0 {
		return 0
	}
	return shift
}
