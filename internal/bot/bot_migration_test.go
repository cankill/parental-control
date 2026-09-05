package bot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"parental-control/internal/lib/types"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		text        string
		wantName    string
		wantPayload string
		wantOK      bool
	}{
		{"/daily", "daily", "", true},
		{" /info@parent_control_bot Google Chrome ", "info", "Google Chrome", true},
		{"/record\t15", "record", "15", true},
		{"daily", "", "", false},
		{"/", "", "", false},
	}
	for _, tt := range tests {
		name, payload, ok := parseCommand(tt.text)
		if name != tt.wantName || payload != tt.wantPayload || ok != tt.wantOK {
			t.Errorf("parseCommand(%q) = (%q,%q,%v), want (%q,%q,%v)", tt.text, name, payload, ok, tt.wantName, tt.wantPayload, tt.wantOK)
		}
	}
}

func TestLegacyCallbackCodec(t *testing.T) {
	encoded := encodeCallbackData("day-prev", "3")
	if encoded != "\fday-prev|3" {
		t.Fatalf("encoded callback = %q", encoded)
	}
	action, payload, ok := parseCallbackData(encoded)
	if !ok || action != "day-prev" || payload != "3" {
		t.Fatalf("parsed callback = (%q,%q,%v)", action, payload, ok)
	}
	if _, _, ok := parseCallbackData("day-prev|3"); ok {
		t.Fatal("non-legacy callback unexpectedly accepted")
	}
}

func TestAdminOnly(t *testing.T) {
	var calls atomic.Int32
	next := func(context.Context, *tgbot.Bot, *models.Update) { calls.Add(1) }
	handler := adminOnly([]int64{42})(next)
	handler(context.Background(), nil, &models.Update{Message: &models.Message{From: &models.User{ID: 42}}})
	handler(context.Background(), nil, &models.Update{Message: &models.Message{From: &models.User{ID: 7}}})
	handler(context.Background(), nil, &models.Update{CallbackQuery: &models.CallbackQuery{From: models.User{ID: 42}}})
	if got := calls.Load(); got != 2 {
		t.Fatalf("allowed handler calls = %d, want 2", got)
	}
}

func TestRenderDailyRich(t *testing.T) {
	resp := &types.AppInfoResponse{
		TimeStamp: "2026-09-02",
		AppInfos: types.AppInfos{
			{Identity: "Safari", Duration: 2 * time.Minute},
			{Identity: "Terminal", Duration: 5 * time.Minute},
		},
		HasOlder: true, OlderShift: 2,
		HasNewer: true, NewerShift: 0,
	}
	message := renderDailyRich(resp)
	if len(message.Blocks) != 3 {
		t.Fatalf("rich blocks = %d, want 3", len(message.Blocks))
	}
	heading := message.Blocks[0].InputRichBlockSectionHeading
	if heading == nil || heading.Size != 6 {
		t.Fatalf("heading = %#v, want smallest size 6", heading)
	}
	table := message.Blocks[1].InputRichBlockTable
	if table == nil || len(table.Cells) != 4 {
		t.Fatalf("table rows = %#v, want header + 2 apps + total", table)
	}
	if !table.IsCompact || table.IsBordered || table.IsStriped {
		t.Fatalf("table density = compact:%v bordered:%v striped:%v", table.IsCompact, table.IsBordered, table.IsStriped)
	}
	if got := table.Cells[0][0].Text.PlainText; got != "App" {
		t.Fatalf("identity header = %q, want App", got)
	}
	if got := table.Cells[0][1].Text.PlainText; got != "Time" {
		t.Fatalf("duration header = %q, want Time", got)
	}
	if got := table.Cells[1][0].Text.PlainText; got != "Terminal" {
		t.Fatalf("first app = %q, want duration-sorted Terminal", got)
	}
	if got := table.Cells[3][1].Text.PlainText; got != "7m 0s" {
		t.Fatalf("total = %q, want 7m 0s", got)
	}
	buttons := message.Blocks[2].InputRichBlockButtons
	if buttons == nil || len(buttons.Buttons) != 2 {
		t.Fatalf("rich buttons = %#v", buttons)
	}
	if got := buttons.Buttons[0].CallbackData; got != "\fday-prev|2" {
		t.Fatalf("previous callback = %q", got)
	}
	if got := buttons.Buttons[1].CallbackData; got != "\fday-next|0" {
		t.Fatalf("next callback = %q", got)
	}
	for _, button := range buttons.Buttons {
		if button.Style != "link" {
			t.Fatalf("button style = %q, want link", button.Style)
		}
	}
	if _, err := json.Marshal(message); err != nil {
		t.Fatalf("marshal rich message: %v", err)
	}
}

func TestRenderHourlyRichCombinesAppsSitesAndNavigation(t *testing.T) {
	apps := &types.AppInfoResponse{
		TimeStamp: "2026-09-05T14",
		AppInfos: types.AppInfos{
			{Identity: "Terminal", Duration: 5 * time.Minute},
		},
		HasOlder: true, OlderShift: 3,
		HasNewer: true, NewerShift: 0,
	}
	sites := &types.AppInfoResponse{
		TimeStamp: "2026-09-05T14",
		AppInfos: types.AppInfos{
			{Identity: "example.com", Duration: 2 * time.Minute},
		},
		HasOlder: true, OlderShift: 2,
	}

	message := renderHourlyRich(apps, sites)
	if len(message.Blocks) != 4 {
		t.Fatalf("rich blocks = %d, want heading, two tables, and one navigation row", len(message.Blocks))
	}
	if got := message.Blocks[0].InputRichBlockSectionHeading.Text.PlainText; got != "Hour: 05.09 14:00" {
		t.Fatalf("heading = %q", got)
	}
	if got := message.Blocks[1].InputRichBlockTable.Cells[0][0].Text.PlainText; got != "App" {
		t.Fatalf("first table identity = %q, want App", got)
	}
	if got := message.Blocks[2].InputRichBlockTable.Cells[0][0].Text.PlainText; got != "Site" {
		t.Fatalf("second table identity = %q, want Site", got)
	}
	buttons := message.Blocks[3].InputRichBlockButtons.Buttons
	if len(buttons) != 2 {
		t.Fatalf("navigation buttons = %#v", buttons)
	}
	if buttons[0].CallbackData != "\fstat-prev|2" || buttons[1].CallbackData != "\fstat-next|0" {
		t.Fatalf("combined navigation = %#v", buttons)
	}
}

func TestRenderHourlyRichEmptyUsesSiteTimeline(t *testing.T) {
	apps := &types.AppInfoResponse{TimeStamp: "2026-09-05T14"}
	sites := &types.AppInfoResponse{TimeStamp: "2026-09-05T14", HasOlder: true, OlderShift: 4}
	message := renderHourlyRich(apps, sites)
	if len(message.Blocks) != 3 {
		t.Fatalf("rich blocks = %d, want heading, empty state, and navigation", len(message.Blocks))
	}
	if got := message.Blocks[1].InputRichBlockParagraph.Text.PlainText; got != "No Statistics" {
		t.Fatalf("empty state = %q", got)
	}
	buttons := message.Blocks[2].InputRichBlockButtons.Buttons
	if len(buttons) != 1 || buttons[0].CallbackData != "\fstat-prev|4" {
		t.Fatalf("combined empty navigation = %#v", buttons)
	}
}

func TestRenderHourlyRichOmitsEmptySitesTable(t *testing.T) {
	apps := &types.AppInfoResponse{
		TimeStamp: "2026-09-05T14",
		AppInfos:  types.AppInfos{{Identity: "Chrome", Duration: time.Minute}},
	}
	message := renderHourlyRich(apps, &types.AppInfoResponse{TimeStamp: apps.TimeStamp})
	if len(message.Blocks) != 2 {
		t.Fatalf("rich blocks = %d, want heading and applications table", len(message.Blocks))
	}
	if got := message.Blocks[1].InputRichBlockTable.Cells[0][0].Text.PlainText; got != "App" {
		t.Fatalf("only table identity = %q, want App", got)
	}
}

func TestRenderDailyAndWeeklyWithSitesUseSharedPeriodNavigation(t *testing.T) {
	dailyApps := &types.AppInfoResponse{
		TimeStamp: "2026-09-05",
		AppInfos:  types.AppInfos{{Identity: "Terminal", Duration: time.Minute}},
		HasOlder:  true, OlderShift: 3,
	}
	dailySites := &types.AppInfoResponse{
		TimeStamp: "2026-09-05",
		AppInfos:  types.AppInfos{{Identity: "example.com", Duration: time.Minute}},
		HasOlder:  true, OlderShift: 2,
	}
	daily := renderDailyWithSites(dailyApps, dailySites)
	if got := daily.Blocks[0].InputRichBlockSectionHeading.Text.PlainText; got != "Day: 05.09" {
		t.Fatalf("daily heading = %q", got)
	}
	dailyButtons := daily.Blocks[len(daily.Blocks)-1].InputRichBlockButtons.Buttons
	if len(dailyButtons) != 1 || dailyButtons[0].CallbackData != "\fday-prev|2" {
		t.Fatalf("daily navigation = %#v", dailyButtons)
	}

	weeklyApps := &types.AppInfoResponse{
		TimeStamp: "2026-08-31 – 2026-09-06",
		AppInfos:  dailyApps.AppInfos,
		HasNewer:  true, NewerShift: 1,
	}
	weeklySites := &types.AppInfoResponse{
		TimeStamp: "2026-08-31 – 2026-09-06",
		AppInfos:  dailySites.AppInfos,
		HasNewer:  true, NewerShift: 0,
	}
	weekly := renderWeeklyWithSites(weeklyApps, weeklySites)
	if got := weekly.Blocks[0].InputRichBlockSectionHeading.Text.PlainText; got != "Week: 31.08 - 06.09" {
		t.Fatalf("weekly heading = %q", got)
	}
	weeklyButtons := weekly.Blocks[len(weekly.Blocks)-1].InputRichBlockButtons.Buttons
	if len(weeklyButtons) != 1 || weeklyButtons[0].CallbackData != "\fweek-next|1" {
		t.Fatalf("weekly navigation = %#v", weeklyButtons)
	}
}

func TestRenderDailyRichWithoutNavigation(t *testing.T) {
	message := renderDailyRich(&types.AppInfoResponse{TimeStamp: "2026-09-02"})
	if len(message.Blocks) != 2 {
		t.Fatalf("rich blocks = %d, want heading and empty-state text", len(message.Blocks))
	}
	paragraph := message.Blocks[1].InputRichBlockParagraph
	if paragraph == nil || paragraph.Text.PlainText != "No Statistics" {
		t.Fatalf("empty-state paragraph = %#v, want No Statistics", paragraph)
	}
}

func TestRenderEmptyStatisticsKeepsNavigation(t *testing.T) {
	message := renderSites(&types.AppInfoResponse{
		TimeStamp:  "2026-09-02T09",
		HasOlder:   true,
		OlderShift: 4,
	})
	if len(message.Blocks) != 3 {
		t.Fatalf("rich blocks = %d, want heading, empty state, and navigation", len(message.Blocks))
	}
	if got := message.Blocks[0].InputRichBlockSectionHeading.Text.PlainText; got != "Sites: 02.09 09:00" {
		t.Fatalf("heading = %q", got)
	}
	if got := message.Blocks[1].InputRichBlockParagraph.Text.PlainText; got != "No Statistics" {
		t.Fatalf("empty state = %q", got)
	}
	buttons := message.Blocks[2].InputRichBlockButtons.Buttons
	if len(buttons) != 1 || buttons[0].CallbackData != "\fsites-prev|4" {
		t.Fatalf("navigation buttons = %#v", buttons)
	}
}

func TestRenderWeeklyRichAndStatsMenu(t *testing.T) {
	message := renderWeeklyRich(&types.AppInfoResponse{
		TimeStamp: "2026-08-31 – 2026-09-06",
		HasOlder:  true, OlderShift: 2,
	})
	if got := message.Blocks[0].InputRichBlockSectionHeading.Text.PlainText; got != "Week: 31.08 - 06.09" {
		t.Fatalf("weekly heading = %q", got)
	}
	buttons := message.Blocks[2].InputRichBlockButtons.Buttons
	if len(buttons) != 1 || buttons[0].CallbackData != "\fweek-prev|2" {
		t.Fatalf("weekly buttons = %#v", buttons)
	}
	if buttons[0].Text.PlainText != "‹" || buttons[0].Style != "link" {
		t.Fatalf("weekly navigation button = %#v", buttons[0])
	}

	menu := renderStatsMenu()
	menuButtons := menu.Blocks[1].InputRichBlockButtons.Buttons
	if len(menuButtons) != 3 {
		t.Fatalf("stats buttons = %d, want hourly, daily, weekly", len(menuButtons))
	}
	want := []string{"\fhub-hourly", "\fhub-daily", "\fhub-weekly"}
	wantText := []string{"Hour", "Day", "Week"}
	for i, button := range menuButtons {
		if button.CallbackData != want[i] {
			t.Fatalf("stats button %d = %q, want %q", i, button.CallbackData, want[i])
		}
		if button.Text.PlainText != wantText[i] || button.Style != "link" {
			t.Fatalf("stats button %d = %#v", i, button)
		}
	}
}

func TestFormatActivityDuration(t *testing.T) {
	tests := map[int]string{
		0:     "No Activity",
		1:     "1 second",
		60:    "1 minute",
		1000:  "16 minutes, 40 seconds",
		3661:  "1 hour, 1 minute, 1 second",
		90061: "1 day, 1 hour, 1 minute, 1 second",
	}
	for seconds, want := range tests {
		if got := formatActivityDuration(seconds); got != want {
			t.Errorf("formatActivityDuration(%d) = %q, want %q", seconds, got, want)
		}
	}
}

func TestCompactDurationAndReportTimestamp(t *testing.T) {
	if got := formatCompactDuration(3*time.Hour + 10*time.Minute + 11731*time.Millisecond); got != "3h 10m 11.731s" {
		t.Fatalf("compact duration = %q", got)
	}
	tests := map[string]string{
		"2026-08-24":                    "24.08",
		"2026-08-24T09":                 "24.08 09:00",
		"2026-08-24 – 2026-08-30":       "24.08 - 30.08",
		"already human-readable period": "already human-readable period",
	}
	for input, want := range tests {
		if got := formatReportTimestamp(input); got != want {
			t.Errorf("formatReportTimestamp(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestEnsureCallbackAnswered(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/answerCallbackQuery") {
			t.Errorf("request path = %q", r.URL.Path)
		}
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	defer server.Close()

	b, err := tgbot.New("123:test", tgbot.WithSkipGetMe(), tgbot.WithServerURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	update := &models.Update{CallbackQuery: &models.CallbackQuery{ID: "callback-id", Data: "unknown"}}
	h := &handlerRegistry{callbacks: map[string]handlerFunc{}}
	h.dispatchCallback(context.Background(), b, update)
	if got := calls.Load(); got != 1 {
		t.Fatalf("answerCallbackQuery calls = %d, want 1", got)
	}
}

func TestSendAndEditDailyRichMessage(t *testing.T) {
	var methods []string
	var replyMessageID int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse form: %v", err)
		}
		methods = append(methods, r.URL.Path)
		var rich models.InputRichMessage
		if err := json.Unmarshal([]byte(r.FormValue("rich_message")), &rich); err != nil {
			t.Errorf("decode rich_message: %v; raw=%q", err, r.FormValue("rich_message"))
		}
		if len(rich.Blocks) != 2 {
			t.Errorf("rich blocks = %d, want 2", len(rich.Blocks))
		}
		if raw := r.FormValue("reply_parameters"); raw != "" {
			var reply models.ReplyParameters
			if err := json.Unmarshal([]byte(raw), &reply); err != nil {
				t.Errorf("decode reply_parameters: %v", err)
			}
			replyMessageID = reply.MessageID
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":9,"date":1,"chat":{"id":42,"type":"private"}}}`))
	}))
	defer server.Close()

	b, err := tgbot.New("123:test", tgbot.WithSkipGetMe(), tgbot.WithServerURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	message := renderDailyRich(&types.AppInfoResponse{TimeStamp: "2026-09-02"})
	rich := newRichClient(server.URL, b.Token(), server.Client())
	command := newUpdateContext(context.Background(), b, &models.Update{
		Message: &models.Message{Chat: models.Chat{ID: 42}},
	}, "", rich)
	if err := command.SendRichMessage(message); err != nil {
		t.Fatalf("send rich message: %v", err)
	}
	if err := rich.send(context.Background(), 42, message, 17); err != nil {
		t.Fatalf("reply with rich message: %v", err)
	}
	callback := newUpdateContext(context.Background(), b, &models.Update{
		CallbackQuery: &models.CallbackQuery{Message: models.MaybeInaccessibleMessage{
			Type:    models.MaybeInaccessibleMessageTypeMessage,
			Message: &models.Message{ID: 9, Chat: models.Chat{ID: 42}},
		}},
	}, "", rich)
	if err := callback.EditRichMessage(message); err != nil {
		t.Fatalf("edit rich message: %v", err)
	}
	if replyMessageID != 17 {
		t.Fatalf("reply message ID = %d, want 17", replyMessageID)
	}
	if len(methods) != 3 || !strings.HasSuffix(methods[0], "/sendRichMessage") || !strings.HasSuffix(methods[1], "/sendRichMessage") || !strings.HasSuffix(methods[2], "/editMessageText") {
		t.Fatalf("Telegram methods = %v", methods)
	}
}

func TestRespondRichMessageSendsForCommandAndEditsForCallback(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart form: %v", err)
		}
		methods = append(methods, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":9}}`))
	}))
	defer server.Close()

	bot, err := tgbot.New("123:test", tgbot.WithSkipGetMe(), tgbot.WithServerURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	client := newRichClient(server.URL, bot.Token(), server.Client())
	message := renderStatus("result")

	command := newUpdateContext(context.Background(), bot, &models.Update{
		Message: &models.Message{ID: 3, Chat: models.Chat{ID: 42}},
	}, "", client)
	if err := command.RespondRichMessage(message); err != nil {
		t.Fatalf("command response: %v", err)
	}

	callback := newUpdateContext(context.Background(), bot, &models.Update{
		CallbackQuery: &models.CallbackQuery{Message: models.MaybeInaccessibleMessage{
			Type:    models.MaybeInaccessibleMessageTypeMessage,
			Message: &models.Message{ID: 9, Chat: models.Chat{ID: 42}},
		}},
	}, "", client)
	if err := callback.RespondRichMessage(message); err != nil {
		t.Fatalf("callback response: %v", err)
	}

	if len(methods) != 2 || !strings.HasSuffix(methods[0], "/sendRichMessage") || !strings.HasSuffix(methods[1], "/editMessageText") {
		t.Fatalf("Telegram methods = %v", methods)
	}
}

func TestSendAndEditRichPhotoMultipart(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart form: %v", err)
		}
		methods = append(methods, r.URL.Path)
		file, _, err := r.FormFile("activity.png")
		if err != nil {
			t.Fatalf("activity attachment: %v", err)
		}
		data, err := io.ReadAll(file)
		_ = file.Close()
		if err != nil || string(data) != "png-data" {
			t.Fatalf("activity attachment = %q, %v", data, err)
		}
		var message models.InputRichMessage
		if err := json.Unmarshal([]byte(r.FormValue("rich_message")), &message); err != nil {
			t.Fatalf("decode rich message: %v", err)
		}
		photo := message.Blocks[0].InputRichBlockPhoto
		if photo == nil || photo.Photo.Media != "attach://activity.png" || photo.Caption.Text.PlainText != "Activity" {
			t.Fatalf("photo block = %#v", photo)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":9}}`))
	}))
	defer server.Close()

	client := newRichClient(server.URL, "123:test", server.Client())
	if err := client.send(context.Background(), 42, renderPhoto(strings.NewReader("png-data"), "activity.png", "Activity"), 0); err != nil {
		t.Fatal(err)
	}
	if err := client.edit(context.Background(), 42, 9, renderPhoto(strings.NewReader("png-data"), "activity.png", "Activity")); err != nil {
		t.Fatal(err)
	}
	if len(methods) != 2 || !strings.HasSuffix(methods[0], "/sendRichMessage") || !strings.HasSuffix(methods[1], "/editMessageText") {
		t.Fatalf("Telegram methods = %v", methods)
	}
}
