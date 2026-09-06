package bot

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"parental-control/internal/lib/types"
	"parental-control/internal/presence"

	"github.com/go-telegram/bot/models"
)

func renderStatistics(resp *types.AppInfoResponse) models.InputRichMessage {
	return renderUsageRich("Hour: "+formatReportTimestamp(resp.TimeStamp), resp, "‹", "stat-prev", "›", "stat-next")
}

func renderHourlyRich(apps, sites *types.AppInfoResponse) models.InputRichMessage {
	return renderStatisticsWithSites("Hour", apps, sites, "stat-prev", "stat-next")
}

func renderDailyWithSites(apps, sites *types.AppInfoResponse) models.InputRichMessage {
	return renderStatisticsWithSites("Day", apps, sites, "day-prev", "day-next")
}

func renderWeeklyWithSites(apps, sites *types.AppInfoResponse) models.InputRichMessage {
	return renderStatisticsWithSites("Week", apps, sites, "week-prev", "week-next")
}

func renderStatisticsWithSites(periodLabel string, apps, sites *types.AppInfoResponse, previousID, nextID string) models.InputRichMessage {
	blocks := []models.InputRichBlock{richHeading(periodLabel + ": " + formatReportTimestamp(apps.TimeStamp))}
	appTable, hasApps := renderUsageTable("App", apps)
	if hasApps {
		blocks = append(blocks, appTable)
	}
	siteTable, hasSites := renderUsageTable("Site", sites)
	if hasSites {
		blocks = append(blocks, siteTable)
	}
	if !hasApps && !hasSites {
		blocks = append(blocks, richParagraph("No Statistics"))
	}

	navigation := combinedNavigation(apps, sites)
	if buttons := makeNavigationRichButtons(navigation, "‹", previousID, "›", nextID); len(buttons) > 0 {
		blocks = append(blocks, richButtons(buttons...))
	}
	return models.InputRichMessage{Blocks: blocks}
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
	blocks := []models.InputRichBlock{richHeading(title)}
	table, hasStatistics := renderUsageTable(identityLabel, resp)
	if hasStatistics {
		blocks = append(blocks, table)
	} else {
		blocks = append(blocks, richParagraph("No Statistics"))
	}
	buttons := makeNavigationRichButtons(resp, previousText, previousID, nextText, nextID)
	if len(buttons) > 0 {
		blocks = append(blocks, richButtons(buttons...))
	}
	return models.InputRichMessage{Blocks: blocks}
}

func renderUsageTable(identityLabel string, resp *types.AppInfoResponse) (models.InputRichBlock, bool) {
	if resp == nil {
		return models.InputRichBlock{}, false
	}
	statistics := make(types.AppInfos, 0, len(resp.AppInfos))
	for _, app := range resp.AppInfos {
		if app.Duration > 0 {
			statistics = append(statistics, app)
		}
	}
	statistics.SortByDurationDesc()
	if len(statistics) == 0 {
		return models.InputRichBlock{}, false
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
			richTableCell(formatTableDuration(app.Duration), false, "right"),
		})
		total += app.Duration
	}
	rows = append(rows, []models.RichBlockTableCell{
		richTableCell("Total", true, "left"),
		richTableCell(formatTableDuration(total), true, "right"),
	})

	return models.InputRichBlock{
		Type: models.RichBlockTypeTable,
		InputRichBlockTable: &models.InputRichBlockTable{
			Cells: rows, IsCompact: true,
		},
	}, true
}

func combinedNavigation(responses ...*types.AppInfoResponse) *types.AppInfoResponse {
	result := &types.AppInfoResponse{}
	for _, resp := range responses {
		if resp == nil {
			continue
		}
		if resp.HasOlder && (!result.HasOlder || resp.OlderShift < result.OlderShift) {
			result.HasOlder = true
			result.OlderShift = resp.OlderShift
		}
		if resp.HasNewer && (!result.HasNewer || resp.NewerShift > result.NewerShift) {
			result.HasNewer = true
			result.NewerShift = resp.NewerShift
		}
	}
	return result
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

func renderPresenceEvent(event presence.Event) models.InputRichMessage {
	switch event.Kind {
	case presence.EventAbsent:
		return renderNotice("Presence alert", "No person detected since "+event.Since.Format("02.01 15:04")+".")
	case presence.EventReturned:
		duration := event.At.Sub(event.Since).Round(time.Second)
		return renderNotice("Presence restored", "Person detected after "+formatCompactDuration(duration)+".")
	default:
		return renderNotice("Presence unavailable", "Camera or local analysis failed repeatedly; absence was not inferred.")
	}
}

func renderPresenceSettings(snapshot presence.Snapshot) models.InputRichMessage {
	policy := snapshot.Policy
	enabled := "Off"
	if policy.Enabled {
		enabled = "On"
	}
	active := "No"
	if snapshot.ActiveNow {
		active = "Yes"
	}

	schedule := "Always"
	switch policy.Mode {
	case presence.ScheduleUntil:
		schedule = "Until " + policy.Until.Format("02.01 15:04")
	case presence.ScheduleDaily:
		schedule = formatPresenceMinute(policy.StartMinute) + " - " + formatPresenceMinute(policy.EndMinute)
	}
	days := "Every day"
	if len(policy.WorkDays) > 0 {
		names := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
		selected := make([]string, 0, len(policy.WorkDays))
		for _, day := range policy.WorkDays {
			if day >= 0 && day < len(names) {
				selected = append(selected, names[day])
			}
		}
		days = strings.Join(selected, ", ")
	}
	text := "Enabled: " + enabled +
		"\nActive now: " + active +
		"\nSchedule: " + schedule +
		"\nDays: " + days +
		"\nCheck: every " + formatCompactDuration(snapshot.Interval) +
		"\nCamera after: " + formatCompactDuration(snapshot.IdleGrace) + " without input" +
		"\nAbsence after: " + strconv.Itoa(snapshot.MissThreshold) + " checks" +
		"\n\nSet: /presence_on [HH:MM | 1d10h30s | HH:MM-HH:MM] [Mon,Tue,Fri]"
	return models.InputRichMessage{Blocks: []models.InputRichBlock{
		richHeading("Presence settings"),
		richParagraph(text),
		richButtons(
			richCallbackButton("Enable", "presence-enable"),
			richCallbackButton("Disable", "presence-disable"),
			richCallbackButton("Check now", "presence-check"),
			richCallbackButton("Today", "presence-report-day"),
			richCallbackButton("Week", "presence-report-week"),
		),
	}}
}

func renderPresenceReport(response *types.PresenceResponse) models.InputRichMessage {
	periodLabel := "Day"
	if response.Period == types.ActivityWeekly {
		periodLabel = "Week"
	}
	blocks := []models.InputRichBlock{
		richHeading("Presence · " + periodLabel + ": " + formatReportTimestamp(response.TimeStamp)),
	}
	if response.MonitoredSeconds() == 0 {
		blocks = append(blocks, richParagraph("No Presence Data"))
	} else {
		text := "Present: " + formatCompactDuration(time.Duration(response.PresentSeconds)*time.Second) +
			"\nAway: " + formatCompactDuration(time.Duration(response.AbsentSeconds)*time.Second) +
			"\nPresence: " + strconv.Itoa(response.PresencePercent()) + "%" +
			"\nAbsences: " + strconv.Itoa(response.AbsenceCount)
		if response.LongestAbsence > 0 {
			text += "\nLongest away: " + formatCompactDuration(time.Duration(response.LongestAbsence)*time.Second)
		}
		if response.UnavailableSeconds > 0 {
			text += "\nUnavailable: " + formatCompactDuration(time.Duration(response.UnavailableSeconds)*time.Second)
		}
		blocks = append(blocks, richParagraph(text))
		if response.Period == types.ActivityWeekly {
			if table, ok := renderPresenceWeekTable(response.Days); ok {
				blocks = append(blocks, table)
			}
		} else if len(response.Days) > 0 {
			if table, ok := renderPresenceAbsenceTable(response.Days[0].Absences); ok {
				blocks = append(blocks, table)
			}
		}
	}
	blocks = append(blocks, richButtons(presenceReportButtons(response)...))
	return models.InputRichMessage{Blocks: blocks}
}

func renderPresenceWeekTable(days []types.PresenceDaySummary) (models.InputRichBlock, bool) {
	rows := [][]models.RichBlockTableCell{
		{richTableCell("Day", true, "left"), richTableCell("Present", true, "right"), richTableCell("Away", true, "right")},
	}
	for _, day := range days {
		if day.MonitoredSeconds() == 0 {
			continue
		}
		parsed, err := time.ParseInLocation("2006-01-02", day.Date, time.Local)
		if err != nil {
			continue
		}
		rows = append(rows, []models.RichBlockTableCell{
			richTableCell(parsed.Format("Mon 02.01"), false, "left"),
			richTableCell(formatTableDuration(time.Duration(day.PresentSeconds)*time.Second)+" · "+strconv.Itoa(day.PresencePercent())+"%", false, "right"),
			richTableCell(formatTableDuration(time.Duration(day.AbsentSeconds)*time.Second), false, "right"),
		})
	}
	if len(rows) == 1 {
		return models.InputRichBlock{}, false
	}
	return models.InputRichBlock{Type: models.RichBlockTypeTable, InputRichBlockTable: &models.InputRichBlockTable{Cells: rows, IsCompact: true}}, true
}

func renderPresenceAbsenceTable(absences []types.PresenceAbsence) (models.InputRichBlock, bool) {
	if len(absences) == 0 {
		return models.InputRichBlock{}, false
	}
	rows := [][]models.RichBlockTableCell{
		{richTableCell("Away", true, "left"), richTableCell("Time", true, "right")},
	}
	start := 0
	const maxRows = 8
	if len(absences) > maxRows {
		start = len(absences) - maxRows
	}
	for _, absence := range absences[start:] {
		rows = append(rows, []models.RichBlockTableCell{
			richTableCell(absence.Start.Format("15:04")+" - "+absence.End.Format("15:04"), false, "left"),
			richTableCell(formatTableDuration(time.Duration(absence.Seconds)*time.Second), false, "right"),
		})
	}
	return models.InputRichBlock{Type: models.RichBlockTypeTable, InputRichBlockTable: &models.InputRichBlockTable{Cells: rows, IsCompact: true}}, true
}

func presenceReportButtons(response *types.PresenceResponse) []models.RichMessageButton {
	buttons := []models.RichMessageButton{
		richCallbackButton("Day", "presence-report-day"),
		richCallbackButton("Week", "presence-report-week"),
	}
	if response.HasOlder {
		buttons = append(buttons, richCallbackButton("‹", "presence-report-prev", presenceReportTarget(response.Period, response.OlderShift)))
	}
	if response.HasNewer {
		buttons = append(buttons, richCallbackButton("›", "presence-report-next", presenceReportTarget(response.Period, response.NewerShift)))
	}
	buttons = append(buttons, richCallbackButton("Settings", "hub-presence-settings"))
	return buttons
}

func formatPresenceMinute(minute int) string {
	if minute == 24*60 {
		return "24:00"
	}
	return fmt.Sprintf("%02d:%02d", minute/60, minute%60)
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

func renderVideo(reader io.Reader, filename, caption string) models.InputRichMessage {
	video := models.InputMediaVideo{
		Media: "attach://" + filename, MediaAttachment: reader,
		Duration: 5, SupportsStreaming: true,
	}
	return models.InputRichMessage{Blocks: []models.InputRichBlock{{
		Type:                models.RichBlockTypeVideo,
		InputRichBlockVideo: &models.InputRichBlockVideo{Video: video, Caption: richCaption(caption)},
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

// Telegram may make the Time column very narrow when a domain is long. Keep a
// compact duration on one line while preserving normal spacing elsewhere.
func formatTableDuration(duration time.Duration) string {
	return strings.ReplaceAll(formatCompactDuration(duration), " ", "\u00a0")
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
