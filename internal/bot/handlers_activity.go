package bot

import (
	"bytes"
	"parental-control/internal/activity"
	"parental-control/internal/lib/types"
	"strconv"
	"strings"
)

const inputMonitoringHelp = "Activity tracking needs Input Monitoring permission. Add /opt/parentcontrol/parent-control in System Settings → Privacy & Security → Input Monitoring, enable it, then restart the ParentControl LaunchAgent."
const inputMonitoringAlert = "Input Monitoring permission is required. Enable it in System Settings, then restart ParentControl."

func (h *handlerRegistry) registerActivityHandlers() {
	h.command("activity", h.sendActivityMenu)
	h.callback("hub-activity", h.hubAction("/activity", h.sendActivityMenu))
	h.callback("activity-hourly", h.selectActivityPeriod(types.ActivityHourly))
	h.callback("activity-daily", h.selectActivityPeriod(types.ActivityDaily))
	h.callback("activity-weekly", h.selectActivityPeriod(types.ActivityWeekly))
	h.callback("activity-prev", h.navigateActivity)
	h.callback("activity-next", h.navigateActivity)
}

func (h *handlerRegistry) sendActivityMenu(c *updateContext) error {
	return c.SendText("Activity:", "", h.keyboards.activityMenu)
}

func (h *handlerRegistry) selectActivityPeriod(period types.ActivityPeriod) handlerFunc {
	return func(c *updateContext) error {
		if err := c.EditText("Activity: "+activityPeriodName(period), "", nil); err != nil {
			return err
		}
		return h.sendActivity(c, period, 0)
	}
}

func (h *handlerRegistry) sendActivity(c *updateContext, period types.ActivityPeriod, shift int) error {
	if !activity.PreflightAccess() {
		activity.RequestAccessOnce()
		return c.SendText(inputMonitoringHelp, "", nil)
	}
	resp, err := h.stats.activity(period, shift)
	if err != nil {
		return c.SendText("Activity unavailable (shutting down)", "", nil)
	}
	data, err := renderActivityPNG(resp)
	if err != nil {
		return c.SendText("Could not render activity chart", "", nil)
	}
	return c.SendPhoto(bytes.NewReader(data), "activity.png", activityCaption(resp), activityKeyboard(resp))
}

func (h *handlerRegistry) navigateActivity(c *updateContext) error {
	if !activity.PreflightAccess() {
		return c.AnswerCallback(inputMonitoringAlert, true)
	}
	period, shift, ok := parseActivityTarget(c.Data())
	if !ok {
		return c.AnswerCallback("Invalid activity period", false)
	}
	resp, err := h.stats.activity(period, shift)
	if err != nil {
		return c.AnswerCallback("Activity unavailable", false)
	}
	data, err := renderActivityPNG(resp)
	if err != nil {
		return c.AnswerCallback("Activity unavailable", false)
	}
	return c.EditPhoto(bytes.NewReader(data), "activity.png", activityCaption(resp), activityKeyboard(resp))
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
