package bot

import (
	"fmt"
	"parental-control/internal/browser"

	"github.com/go-telegram/bot/models"
)

func (h *handlerRegistry) registerWebHandlers() {
	h.command("url", h.sendURL)
	h.command("sites", h.sendSites)
	h.command("web", func(c *updateContext) error {
		return c.SendText("Web:", "", h.keyboards.web)
	})
	h.callback("hub-url", h.hubAction("/url", h.sendURL))
	h.callback("hub-sites", h.hubAction("/sites", h.sendSites))
	h.callback("sites-prev", h.navigateSites)
	h.callback("sites-next", h.navigateSites)
}

func (h *handlerRegistry) sendURL(c *updateContext) error {
	url, err := browser.FrontmostBrowserURL()
	if err != nil {
		return c.SendText(fmt.Sprintf("No browser URL: %s", err), "", nil)
	}
	if url == "" {
		return c.SendText("No active browser tab", "", nil)
	}
	return c.SendText(url, "", nil)
}

func (h *handlerRegistry) sendSites(c *updateContext) error {
	resp, err := h.stats.sites(0)
	if err != nil {
		return c.SendText("Statistics unavailable (shutting down)", "", nil)
	}
	text, kb := renderSites(resp)
	return c.SendText(text, models.ParseModeMarkdown, kb)
}

func (h *handlerRegistry) navigateSites(c *updateContext) error {
	resp, err := h.stats.sites(callbackShift(c))
	if err != nil {
		return err
	}
	text, kb := renderSites(resp)
	return c.EditText(text, models.ParseModeMarkdown, kb)
}
