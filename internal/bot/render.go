package bot

import (
	"io"
	"parental-control/internal/lib/types"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"
)

func renderStatistics(resp *types.AppInfoResponse) models.InputRichMessage {
	return renderUsageRich("Hour: "+formatReportTimestamp(resp.TimeStamp), resp, "‹", "stat-prev", "›", "stat-next")
}

func renderDailyRich(resp *types.AppInfoResponse) models.InputRichMessage {
	return renderUsageRich("Day: "+formatReportTimestamp(resp.TimeStamp), resp, "‹", "day-prev", "›", "day-next")
}

func renderWeeklyRich(resp *types.AppInfoResponse) models.InputRichMessage {
	return renderUsageRich("Week: "+formatReportTimestamp(resp.TimeStamp), resp, "‹", "week-prev", "›", "week-next")
}

func renderSites(resp *types.AppInfoResponse) models.InputRichMessage {
	return renderUsageRichWithLabel("Sites: "+formatReportTimestamp(resp.TimeStamp), "Site", resp, "‹", "sites-prev", "›", "sites-next")
}

func renderUsageRich(title string, resp *types.AppInfoResponse, previousText, previousID, nextText, nextID string) models.InputRichMessage {
	return renderUsageRichWithLabel(title, "App", resp, previousText, previousID, nextText, nextID)
}

func renderUsageRichWithLabel(title, identityLabel string, resp *types.AppInfoResponse, previousText, previousID, nextText, nextID string) models.InputRichMessage {
	statistics := make(types.AppInfos, 0, len(resp.AppInfos))
	for _, app := range resp.AppInfos {
		if app.Duration > 0 {
			statistics = append(statistics, app)
		}
	}
	statistics.SortByDurationDesc()
	blocks := []models.InputRichBlock{richHeading(title)}
	if len(statistics) == 0 {
		blocks = append(blocks, richParagraph("No Statistics"))
		buttons := makeNavigationRichButtons(resp, previousText, previousID, nextText, nextID)
		if len(buttons) > 0 {
			blocks = append(blocks, richButtons(buttons...))
		}
		return models.InputRichMessage{Blocks: blocks}
	}

	rows := [][]models.RichBlockTableCell{
		{richTableCell(identityLabel, true, "left"), richTableCell("Time", true, "right")},
	}
	total := time.Duration(0)
	for _, app := range statistics {
		name := app.Identity
		if resp.ActiveApp != "" && app.Identity == resp.ActiveApp {
			name = "● " + name
		}
		rows = append(rows, []models.RichBlockTableCell{
			richTableCell(name, false, "left"),
			richTableCell(formatCompactDuration(app.Duration), false, "right"),
		})
		total += app.Duration
	}
	rows = append(rows, []models.RichBlockTableCell{
		richTableCell("Total", true, "left"),
		richTableCell(formatCompactDuration(total), true, "right"),
	})

	blocks = append(blocks,
		models.InputRichBlock{
			Type: models.RichBlockTypeTable,
			InputRichBlockTable: &models.InputRichBlockTable{
				Cells: rows, IsCompact: true,
			},
		},
	)
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
	return renderPhotoWithRichCaption(reader, filename, richText(caption), buttons...)
}

func renderPhotoWithRichCaption(reader io.Reader, filename string, caption models.RichText, buttons ...models.RichMessageButton) models.InputRichMessage {
	photo := models.InputMediaPhoto{Media: "attach://" + filename, MediaAttachment: reader}
	block := models.InputRichBlock{
		Type:                models.RichBlockTypePhoto,
		InputRichBlockPhoto: &models.InputRichBlockPhoto{Photo: photo, Caption: richCaptionText(caption)},
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

func richCaptionText(text models.RichText) *models.RichBlockCaption {
	return &models.RichBlockCaption{Text: text}
}

func richText(text string) models.RichText {
	return models.RichText{PlainText: text}
}

func richBold(text string) models.RichText {
	return models.RichText{
		Type: models.RichTextTypeBold,
		RichTextBold: &models.RichTextBold{
			Text: richText(text),
		},
	}
}

func richTextSequence(parts ...models.RichText) models.RichText {
	return models.RichText{Array: parts}
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

func formatCompactDuration(duration time.Duration) string {
	value := duration.String()
	var formatted strings.Builder
	formatted.Grow(len(value) + 3)
	var previous rune
	for _, r := range value {
		if r >= '0' && r <= '9' {
			if previous == 'h' || previous == 'm' || previous == 's' || previous == 'µ' {
				formatted.WriteByte(' ')
			}
		}
		formatted.WriteRune(r)
		previous = r
	}
	return formatted.String()
}

func formatReportTimestamp(value string) string {
	if parts := strings.SplitN(value, " - ", 2); len(parts) == 2 {
		return formatReportTimestamp(strings.TrimSpace(parts[0])) + " - " + formatReportTimestamp(strings.TrimSpace(parts[1]))
	}
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == '–' })
	if len(parts) == 2 {
		return formatReportTimestamp(strings.TrimSpace(parts[0])) + " - " + formatReportTimestamp(strings.TrimSpace(parts[1]))
	}
	for _, layout := range []string{"2006-01-02T15", "2006-01-02"} {
		parsed, err := time.ParseInLocation(layout, strings.TrimSpace(value), time.Local)
		if err != nil {
			continue
		}
		if layout == "2006-01-02T15" {
			return parsed.Format("02.01 15:00")
		}
		return parsed.Format("02.01")
	}
	return value
}
