package bot

import (
	"bytes"
	"parental-control/internal/activity"
	"parental-control/internal/lib/types"
	"strconv"
	"strings"

	tele "gopkg.in/telebot.v4"
)

const inputMonitoringHelp = "Activity tracking needs Input Monitoring permission. Add /opt/parentcontrol/parent-control in System Settings → Privacy & Security → Input Monitoring, enable it, then restart the ParentControl LaunchAgent."
const inputMonitoringAlert = "Input Monitoring permission is required. Enable it in System Settings, then restart ParentControl."

func (h *handlerRegistry) registerActivityHandlers() {
	h.bot.Handle("/activity", h.sendActivityMenu)
	h.bot.Handle(&h.keyboards.activity, h.hubAction("/activity", h.sendActivityMenu))
	h.bot.Handle(&h.keyboards.activityHourly, h.selectActivityPeriod(types.ActivityHourly))
	h.bot.Handle(&h.keyboards.activityDaily, h.selectActivityPeriod(types.ActivityDaily))
	h.bot.Handle(&h.keyboards.activityWeekly, h.selectActivityPeriod(types.ActivityWeekly))
	h.bot.Handle(&h.keyboards.activityPrev, h.navigateActivity)
	h.bot.Handle(&h.keyboards.activityNext, h.navigateActivity)
}

func (h *handlerRegistry) sendActivityMenu(c tele.Context) error {
	return c.Send("Activity:", h.keyboards.activityMenu)
}

func (h *handlerRegistry) selectActivityPeriod(period types.ActivityPeriod) tele.HandlerFunc {
	return func(c tele.Context) error {
		_ = c.Edit("Activity: " + activityPeriodName(period))
		err := h.sendActivity(c, period, 0)
		_ = c.Respond()
		return err
	}
}

func (h *handlerRegistry) sendActivity(c tele.Context, period types.ActivityPeriod, shift int) error {
	if !activity.PreflightAccess() {
		activity.RequestAccessOnce()
		return c.Send(inputMonitoringHelp)
	}
	resp, err := h.stats.activity(period, shift)
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
	period, shift, ok := parseActivityTarget(c.Data())
	if !ok {
		return c.Respond(&tele.CallbackResponse{Text: "Invalid activity period"})
	}
	resp, err := h.stats.activity(period, shift)
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

func parseActivityTarget(data string) (types.ActivityPeriod, int, bool) {
	periodText, shiftText, ok := strings.Cut(data, ":")
	if !ok {
		return 0, 0, false
	}
	periodValue, err := strconv.Atoi(periodText)
	if err != nil || periodValue < int(types.ActivityHourly) || periodValue > int(types.ActivityWeekly) {
		return 0, 0, false
	}
	shift, err := strconv.Atoi(shiftText)
	if err != nil || shift < 0 {
		return 0, 0, false
	}
	return types.ActivityPeriod(periodValue), shift, true
}
