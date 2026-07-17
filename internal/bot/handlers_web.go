package bot

import (
	"fmt"
	"parental-control/internal/browser"

	tele "gopkg.in/telebot.v4"
)

func (h *handlerRegistry) registerWebHandlers() {
	h.bot.Handle("/url", h.sendURL)
	h.bot.Handle("/sites", h.sendSites)
	h.bot.Handle("/web", func(c tele.Context) error {
		return c.Send("Web:", h.keyboards.web)
	})
	h.bot.Handle(&h.keyboards.url, h.hubAction("/url", h.sendURL))
	h.bot.Handle(&h.keyboards.sites, h.hubAction("/sites", h.sendSites))
	h.bot.Handle(&h.keyboards.sitesPrev, h.navigateSites)
	h.bot.Handle(&h.keyboards.sitesNext, h.navigateSites)
}

func (h *handlerRegistry) sendURL(c tele.Context) error {
	url, err := browser.FrontmostBrowserURL()
	if err != nil {
		return c.Send(fmt.Sprintf("No browser URL: %s", err))
	}
	if url == "" {
		return c.Send("No active browser tab")
	}
	return c.Send(url)
}

func (h *handlerRegistry) sendSites(c tele.Context) error {
	resp, err := h.stats.sites(0)
	if err != nil {
		return c.Send("Statistics unavailable (shutting down)")
	}
	text, kb := renderSites(resp)
	return c.Send(text, markdown(kb))
}

func (h *handlerRegistry) navigateSites(c tele.Context) error {
	resp, err := h.stats.sites(callbackShift(c))
	if err == nil {
		text, kb := renderSites(resp)
		_ = c.Edit(text, markdown(kb))
	}
	return c.Respond()
}
