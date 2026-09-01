package bot

import (
	"parental-control/internal/lib/types"
	"strconv"
	"time"

	"github.com/go-telegram/bot/models"
)

func renderStatistics(resp *types.AppInfoResponse) (string, *models.InlineKeyboardMarkup) {
	resp.AppInfos.SortByDurationDesc()
	text := "```\n" + "  For: " + resp.TimeStamp + "\n\n" + resp.AppInfos.FormatTableMarked(resp.ActiveApp) + "\n```"
	return text, makeHourKeyboard(resp, "stat-prev", "stat-next")
}

func renderDaily(resp *types.AppInfoResponse) (string, *models.InlineKeyboardMarkup) {
	resp.AppInfos.SortByDurationDesc()
	text := "```\n" + "  Day: " + resp.TimeStamp + "\n\n" + resp.AppInfos.FormatTable() + "\n```"
	return text, makeNavigationKeyboard(resp, "‹ Prev day", "day-prev", "Next day ›", "day-next")
}

func renderDailyRich(resp *types.AppInfoResponse) models.InputRichMessage {
	resp.AppInfos.SortByDurationDesc()
	rows := [][]models.RichBlockTableCell{
		{richTableCell("Application", true, "left"), richTableCell("Time spent", true, "right")},
	}
	total := time.Duration(0)
	for _, app := range resp.AppInfos {
		rows = append(rows, []models.RichBlockTableCell{
			richTableCell(app.Identity, false, "left"),
			richTableCell(app.Duration.String(), false, "right"),
		})
		total += app.Duration
	}
	rows = append(rows, []models.RichBlockTableCell{
		richTableCell("Total", true, "left"),
		richTableCell(total.String(), true, "right"),
	})

	blocks := []models.InputRichBlock{
		{
			Type: models.RichBlockTypeSectionHeading,
			InputRichBlockSectionHeading: &models.InputRichBlockSectionHeading{
				Text: richText("Day: " + resp.TimeStamp),
				Size: 2,
			},
		},
		{
			Type: models.RichBlockTypeTable,
			InputRichBlockTable: &models.InputRichBlockTable{
				Cells: rows, IsBordered: true, IsStriped: true, IsCompact: true,
			},
		},
	}

	buttons := makeNavigationRichButtons(resp, "‹ Prev day", "day-prev", "Next day ›", "day-next")
	if len(buttons) > 0 {
		blocks = append(blocks, models.InputRichBlock{
			Type: models.RichBlockTypeButtons,
			InputRichBlockButtons: &models.InputRichBlockButtons{
				Buttons: buttons, Align: "center",
			},
		})
	}
	return models.InputRichMessage{Blocks: blocks}
}

func richText(text string) models.RichText {
	return models.RichText{PlainText: text}
}

func richTableCell(text string, header bool, align string) models.RichBlockTableCell {
	value := richText(text)
	return models.RichBlockTableCell{Text: &value, IsHeader: header, Align: align, Valign: "middle"}
}

func makeNavigationRichButtons(resp *types.AppInfoResponse, previousText, previousID, nextText, nextID string) []models.RichMessageButton {
	buttons := make([]models.RichMessageButton, 0, 2)
	if resp.HasOlder {
		buttons = append(buttons, models.RichMessageButton{
			Text: richText(previousText), Style: "primary", CallbackData: encodeCallbackData(previousID, strconv.Itoa(resp.OlderShift)),
		})
	}
	if resp.HasNewer {
		buttons = append(buttons, models.RichMessageButton{
			Text: richText(nextText), Style: "primary", CallbackData: encodeCallbackData(nextID, strconv.Itoa(resp.NewerShift)),
		})
	}
	return buttons
}

func renderSites(resp *types.AppInfoResponse) (string, *models.InlineKeyboardMarkup) {
	resp.AppInfos.SortByDurationDesc()
	text := "```\n" + "  Sites for: " + resp.TimeStamp + "\n\n" + resp.AppInfos.FormatTableMarked(resp.ActiveApp) + "\n```"
	return text, makeHourKeyboard(resp, "sites-prev", "sites-next")
}

func makeHourKeyboard(resp *types.AppInfoResponse, previous, next string) *models.InlineKeyboardMarkup {
	return makeNavigationKeyboard(resp, "‹ Earlier", previous, "Later ›", next)
}

func makeNavigationKeyboard(resp *types.AppInfoResponse, previousText, previousID, nextText, nextID string) *models.InlineKeyboardMarkup {
	buttons := make([]models.InlineKeyboardButton, 0, 2)
	if resp.HasOlder {
		buttons = append(buttons, callbackButton(previousText, previousID, strconv.Itoa(resp.OlderShift)))
	}
	if resp.HasNewer {
		buttons = append(buttons, callbackButton(nextText, nextID, strconv.Itoa(resp.NewerShift)))
	}
	if len(buttons) == 0 {
		return nil
	}
	return inlineKeyboard(buttons...)
}
