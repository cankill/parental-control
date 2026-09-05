package bot

import "parental-control/internal/browser"

func (h *handlerRegistry) registerWebHandlers() {
	h.command("url", h.sendURL)
	h.command("sites", h.sendSites)
	h.command("web", func(c *updateContext) error {
		return c.SendRichMessage(renderMenu("Web",
			richCallbackButton("URL", "hub-url"),
			richCallbackButton("Sites", "hub-sites"),
		))
	})
	h.callback("hub-url", h.hubAction("/url", h.sendURL))
	h.callback("hub-sites", h.hubAction("/sites", h.sendSites))
	h.callback("sites-prev", h.navigateSites)
	h.callback("sites-next", h.navigateSites)
}

func (h *handlerRegistry) sendURL(c *updateContext) error {
	url, err := browser.FrontmostBrowserURL()
	if err != nil {
		return c.SendRichMessage(renderNotice("Browser URL unavailable", err.Error()))
	}
	if url == "" {
		return c.SendRichMessage(renderNotice("Browser", "No active browser tab."))
	}
	return c.SendRichMessage(renderNotice("Current browser URL", url))
}

func (h *handlerRegistry) sendSites(c *updateContext) error {
	resp, err := h.stats.sites(0)
	if err != nil {
		return c.SendRichMessage(renderNotice("Site statistics unavailable", "ParentControl is shutting down."))
	}
	return c.SendRichMessage(renderSites(resp))
}

func (h *handlerRegistry) navigateSites(c *updateContext) error {
	resp, err := h.stats.sites(callbackShift(c))
	if err != nil {
		return err
	}
	return c.EditRichMessage(renderSites(resp))
}
