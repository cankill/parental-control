package bot

import (
	"parental-control/internal/lib/types"
	"strconv"

	tele "gopkg.in/telebot.v4"
)

func renderStatistics(resp *types.AppInfoResponse) (string, *tele.ReplyMarkup) {
	resp.AppInfos.SortByDurationDesc()
	text := "```\n" + "  For: " + resp.TimeStamp + "\n\n" + resp.AppInfos.FormatTableMarked(resp.ActiveApp) + "\n```"
	return text, makeHourKeyboard(resp, "stat-prev", "stat-next")
}

func renderDaily(resp *types.AppInfoResponse) (string, *tele.ReplyMarkup) {
	resp.AppInfos.SortByDurationDesc()
	text := "```\n" + "  Day: " + resp.TimeStamp + "\n\n" + resp.AppInfos.FormatTable() + "\n```"
	return text, makeNavigationKeyboard(resp, "‹ Prev day", "day-prev", "Next day ›", "day-next")
}

func renderSites(resp *types.AppInfoResponse) (string, *tele.ReplyMarkup) {
	resp.AppInfos.SortByDurationDesc()
	text := "```\n" + "  Sites for: " + resp.TimeStamp + "\n\n" + resp.AppInfos.FormatTableMarked(resp.ActiveApp) + "\n```"
	return text, makeHourKeyboard(resp, "sites-prev", "sites-next")
}

func makeHourKeyboard(resp *types.AppInfoResponse, previous, next string) *tele.ReplyMarkup {
	return makeNavigationKeyboard(resp, "‹ Earlier", previous, "Later ›", next)
}

func makeNavigationKeyboard(resp *types.AppInfoResponse, previousText, previousID, nextText, nextID string) *tele.ReplyMarkup {
	kb := &tele.ReplyMarkup{}
	buttons := []tele.Btn{}
	if resp.HasOlder {
		buttons = append(buttons, kb.Data(previousText, previousID, strconv.Itoa(resp.OlderShift)))
	}
	if resp.HasNewer {
		buttons = append(buttons, kb.Data(nextText, nextID, strconv.Itoa(resp.NewerShift)))
	}
	if len(buttons) > 0 {
		kb.Inline(kb.Row(buttons...))
	}
	return kb
}
