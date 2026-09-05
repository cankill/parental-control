package bot

import (
	"io"
	"parental-control/internal/lib/types"
	"strconv"
	"time"

	"github.com/go-telegram/bot/models"
)

func renderStatistics(resp *types.AppInfoResponse) models.InputRichMessage {
	return renderUsageRich("Hour "+resp.TimeStamp, resp, "‹", "stat-prev", "›", "stat-next")
}

func renderDailyRich(resp *types.AppInfoResponse) models.InputRichMessage {
	return renderUsageRich("Day "+resp.TimeStamp, resp, "‹", "day-prev", "›", "day-next")
}

func renderWeeklyRich(resp *types.AppInfoResponse) models.InputRichMessage {
	return renderUsageRich("Week "+resp.TimeStamp, resp, "‹", "week-prev", "›", "week-next")
}

func renderSites(resp *types.AppInfoResponse) models.InputRichMessage {
	return renderUsageRichWithLabel("Sites "+resp.TimeStamp, "Site", resp, "‹", "sites-prev", "›", "sites-next")
}

func renderUsageRich(title string, resp *types.AppInfoResponse, previousText, previousID, nextText, nextID string) models.InputRichMessage {
	return renderUsageRichWithLabel(title, "App", resp, previousText, previousID, nextText, nextID)
}

func renderUsageRichWithLabel(title, identityLabel string, resp *types.AppInfoResponse, previousText, previousID, nextText, nextID string) models.InputRichMessage {
	resp.AppInfos.SortByDurationDesc()
	rows := [][]models.RichBlockTableCell{
		{richTableCell(identityLabel, true, "left"), richTableCell("Time", true, "right")},
	}
	total := time.Duration(0)
	for _, app := range resp.AppInfos {
		name := app.Identity
		if resp.ActiveApp != "" && app.Identity == resp.ActiveApp {
			name = "● " + name
		}
		rows = append(rows, []models.RichBlockTableCell{
			richTableCell(name, false, "left"),
			richTableCell(app.Duration.String(), false, "right"),
		})
		total += app.Duration
	}
	rows = append(rows, []models.RichBlockTableCell{
		richTableCell("Total", true, "left"),
		richTableCell(total.String(), true, "right"),
	})

	blocks := []models.InputRichBlock{
		richHeading(title),
		{
			Type: models.RichBlockTypeTable,
			InputRichBlockTable: &models.InputRichBlockTable{
				Cells: rows, IsCompact: true,
			},
		},
	}
	buttons := makeNavigationRichButtons(resp, previousText, previousID, nextText, nextID)
	if len(buttons) > 0 {
		blocks = append(blocks, richButtons(buttons...))
	}
	return models.InputRichMessage{Blocks: blocks}
}

func renderMenu(title string, buttons ...models.RichMessageButton) models.InputRichMessage {
	return models.InputRichMessage{Blocks: []models.InputRichBlock{richHeading(title), richButtons(buttons...)}}
}

func renderStatsMenu() models.InputRichMessage {
	return renderMenu("Statistics",
		richCallbackButton("Hour", "hub-hourly"),
		richCallbackButton("Day", "hub-daily"),
		richCallbackButton("Week", "hub-weekly"),
	)
}

func renderActivityMenu() models.InputRichMessage {
	return renderMenu("Activity",
		richCallbackButton("Hour", "activity-hourly"),
		richCallbackButton("Day", "activity-daily"),
		richCallbackButton("Week", "activity-weekly"),
	)
}

func renderStatus(text string) models.InputRichMessage {
	return models.InputRichMessage{Blocks: []models.InputRichBlock{richParagraph(text)}}
}

func renderNotice(title, text string) models.InputRichMessage {
	blocks := []models.InputRichBlock{richHeading(title)}
	if text != "" {
		blocks = append(blocks, richParagraph(text))
	}
	return models.InputRichMessage{Blocks: blocks}
}

func renderPreformatted(title, text string) models.InputRichMessage {
	return models.InputRichMessage{Blocks: []models.InputRichBlock{
		richHeading(title),
		{Type: models.RichBlockTypePreformatted, InputRichBlockPreformatted: &models.InputRichBlockPreformatted{Text: richText(text)}},
	}}
}

func renderPhoto(reader io.Reader, filename, caption string, buttons ...models.RichMessageButton) models.InputRichMessage {
	photo := models.InputMediaPhoto{Media: "attach://" + filename, MediaAttachment: reader}
	block := models.InputRichBlock{
		Type:                models.RichBlockTypePhoto,
		InputRichBlockPhoto: &models.InputRichBlockPhoto{Photo: photo, Caption: richCaption(caption)},
	}
	blocks := []models.InputRichBlock{block}
	if len(buttons) > 0 {
		blocks = append(blocks, richButtons(buttons...))
	}
	return models.InputRichMessage{Blocks: blocks}
}

func renderAudio(reader io.Reader, filename, caption string) models.InputRichMessage {
	audio := models.InputMediaAudio{Media: "attach://" + filename, MediaAttachment: reader}
	return models.InputRichMessage{Blocks: []models.InputRichBlock{{
		Type:                models.RichBlockTypeAudio,
		InputRichBlockAudio: &models.InputRichBlockAudio{Audio: audio, Caption: richCaption(caption)},
	}}}
}

func richCaption(text string) *models.RichBlockCaption {
	if text == "" {
		return nil
	}
	return &models.RichBlockCaption{Text: richText(text)}
}

func richText(text string) models.RichText {
	return models.RichText{PlainText: text}
}

func richHeading(text string) models.InputRichBlock {
	return models.InputRichBlock{
		Type:                         models.RichBlockTypeSectionHeading,
		InputRichBlockSectionHeading: &models.InputRichBlockSectionHeading{Text: richText(text), Size: 6},
	}
}

func richParagraph(text string) models.InputRichBlock {
	return models.InputRichBlock{
		Type:                    models.RichBlockTypeParagraph,
		InputRichBlockParagraph: &models.InputRichBlockParagraph{Text: richText(text)},
	}
}

func richButtons(buttons ...models.RichMessageButton) models.InputRichBlock {
	return models.InputRichBlock{
		Type:                  models.RichBlockTypeButtons,
		InputRichBlockButtons: &models.InputRichBlockButtons{Buttons: buttons, Align: "center"},
	}
}

func richTableCell(text string, header bool, align string) models.RichBlockTableCell {
	value := richText(text)
	return models.RichBlockTableCell{Text: &value, IsHeader: header, Align: align, Valign: "middle"}
}

func richCallbackButton(text, action string, payload ...string) models.RichMessageButton {
	return models.RichMessageButton{Text: richText(text), Style: "link", CallbackData: encodeCallbackData(action, payload...)}
}

func makeNavigationRichButtons(resp *types.AppInfoResponse, previousText, previousID, nextText, nextID string) []models.RichMessageButton {
	buttons := make([]models.RichMessageButton, 0, 2)
	if resp.HasOlder {
		buttons = append(buttons, richCallbackButton(previousText, previousID, strconv.Itoa(resp.OlderShift)))
	}
	if resp.HasNewer {
		buttons = append(buttons, richCallbackButton(nextText, nextID, strconv.Itoa(resp.NewerShift)))
	}
	return buttons
}
