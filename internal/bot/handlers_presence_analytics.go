package bot

import (
	"parental-control/internal/lib/types"
	"strconv"
	"strings"
)

func (h *handlerRegistry) selectPresenceReport(period types.ActivityPeriod) handlerFunc {
	return func(c *updateContext) error {
		if err := c.AnswerCallback("", false); err != nil {
			return err
		}
		return h.sendPresenceReport(c, period, 0)
	}
}

func (h *handlerRegistry) navigatePresenceReport(c *updateContext) error {
	period, shift, ok := parsePresenceReportTarget(c.Data())
	if !ok {
		return c.AnswerCallback("Invalid presence period", false)
	}
	return h.sendPresenceReport(c, period, shift)
}

func (h *handlerRegistry) sendPresenceReport(c *updateContext, period types.ActivityPeriod, shift int) error {
	response, err := h.stats.presence(period, shift)
	if err != nil {
		return c.RespondRichMessage(renderNotice("Presence analytics unavailable", "ParentControl is shutting down."))
	}
	return c.RespondRichMessage(renderPresenceReport(response))
}

func presenceReportTarget(period types.ActivityPeriod, shift int) string {
	return strconv.Itoa(int(period)) + ":" + strconv.Itoa(shift)
}

func parsePresenceReportTarget(data string) (types.ActivityPeriod, int, bool) {
	periodText, shiftText, ok := strings.Cut(data, ":")
	if !ok {
		return 0, 0, false
	}
	periodValue, err := strconv.Atoi(periodText)
	if err != nil || (periodValue != int(types.ActivityDaily) && periodValue != int(types.ActivityWeekly)) {
		return 0, 0, false
	}
	shift, err := strconv.Atoi(shiftText)
	if err != nil || shift < 0 {
		return 0, 0, false
	}
	return types.ActivityPeriod(periodValue), shift, true
}
