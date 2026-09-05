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
	h.callback("hub-url", h.hubAction(h.sendURL))
	h.callback("hub-sites", h.hubAction(h.sendSites))
	h.callback("sites-prev", h.navigateSites)
	h.callback("sites-next", h.navigateSites)
}

func (h *handlerRegistry) sendURL(c *updateContext) error {
	url, err := browser.FrontmostBrowserURL()
	if err != nil {
		return c.RespondRichMessage(renderNotice("Browser URL unavailable", err.Error()))
	}
	if url == "" {
		return c.RespondRichMessage(renderNotice("Browser", "No active browser tab."))
	}
	return c.RespondRichMessage(renderNotice("Current browser URL", url))
}

func (h *handlerRegistry) sendSites(c *updateContext) error {
	resp, err := h.stats.sites(0)
	if err != nil {
		return c.RespondRichMessage(renderNotice("Site statistics unavailable", "ParentControl is shutting down."))
	}
	return c.RespondRichMessage(renderSites(resp))
}

func (h *handlerRegistry) navigateSites(c *updateContext) error {
	resp, err := h.stats.sites(callbackShift(c))
	if err != nil {
		return err
	}
	return c.EditRichMessage(renderSites(resp))
}
