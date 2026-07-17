package bot

import (
	"bytes"
	"parental-control/internal/activity"

	tele "gopkg.in/telebot.v4"
)

const inputMonitoringHelp = "Activity tracking needs Input Monitoring permission. Add /opt/parentcontrol/parent-control in System Settings → Privacy & Security → Input Monitoring, enable it, then restart the ParentControl LaunchAgent."
const inputMonitoringAlert = "Input Monitoring permission is required. Enable it in System Settings, then restart ParentControl."

func (h *handlerRegistry) registerActivityHandlers() {
	h.bot.Handle("/activity", h.sendActivity)
	h.bot.Handle(&h.keyboards.activity, h.hubAction("/activity", h.sendActivity))
	h.bot.Handle(&h.keyboards.activityPrev, h.navigateActivity)
	h.bot.Handle(&h.keyboards.activityNext, h.navigateActivity)
}

func (h *handlerRegistry) sendActivity(c tele.Context) error {
	if !activity.PreflightAccess() {
		activity.RequestAccessOnce()
		return c.Send(inputMonitoringHelp)
	}
	resp, err := h.stats.activity(0)
	if err != nil {
		return c.Send("Activity unavailable (shutting down)")
	}
	data, err := renderActivityPNG(resp)
	if err != nil {
		return c.Send("Could not render activity chart")
	}
	photo := &tele.Photo{File: tele.FromReader(bytes.NewReader(data)), Caption: activityCaption(resp)}
	return c.Send(photo, &tele.SendOptions{ReplyMarkup: activityKeyboard(resp)})
}

func (h *handlerRegistry) navigateActivity(c tele.Context) error {
	if !activity.PreflightAccess() {
		return c.Respond(&tele.CallbackResponse{Text: inputMonitoringAlert, ShowAlert: true})
	}
	resp, err := h.stats.activity(callbackShift(c))
	if err == nil {
		data, renderErr := renderActivityPNG(resp)
		if renderErr == nil {
			photo := &tele.Photo{File: tele.FromReader(bytes.NewReader(data)), Caption: activityCaption(resp)}
			_, err = h.bot.EditMedia(c.Message(), photo, &tele.SendOptions{ReplyMarkup: activityKeyboard(resp)})
		}
	}
	if err != nil {
		return c.Respond(&tele.CallbackResponse{Text: "Activity unavailable"})
	}
	return c.Respond()
}
