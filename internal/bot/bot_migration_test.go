package bot

import (
	"context"
	"encoding/json"
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
	table := message.Blocks[1].InputRichBlockTable
	if table == nil || len(table.Cells) != 4 {
		t.Fatalf("table rows = %#v, want header + 2 apps + total", table)
	}
	if got := table.Cells[1][0].Text.PlainText; got != "Terminal" {
		t.Fatalf("first app = %q, want duration-sorted Terminal", got)
	}
	if got := table.Cells[3][1].Text.PlainText; got != "7m0s" {
		t.Fatalf("total = %q, want 7m0s", got)
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
	if _, err := json.Marshal(message); err != nil {
		t.Fatalf("marshal rich message: %v", err)
	}
}

func TestRenderDailyRichWithoutNavigation(t *testing.T) {
	message := renderDailyRich(&types.AppInfoResponse{TimeStamp: "2026-09-02"})
	if len(message.Blocks) != 2 {
		t.Fatalf("rich blocks = %d, want heading and table", len(message.Blocks))
	}
	table := message.Blocks[1].InputRichBlockTable
	if table == nil || len(table.Cells) != 2 {
		t.Fatalf("empty table rows = %#v, want header and zero total", table)
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
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":9,"date":1,"chat":{"id":42,"type":"private"}}}`))
	}))
	defer server.Close()

	b, err := tgbot.New("123:test", tgbot.WithSkipGetMe(), tgbot.WithServerURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	message := renderDailyRich(&types.AppInfoResponse{TimeStamp: "2026-09-02"})
	command := newUpdateContext(context.Background(), b, &models.Update{
		Message: &models.Message{Chat: models.Chat{ID: 42}},
	}, "")
	if err := command.SendRichMessage(message); err != nil {
		t.Fatalf("send rich message: %v", err)
	}
	callback := newUpdateContext(context.Background(), b, &models.Update{
		CallbackQuery: &models.CallbackQuery{Message: models.MaybeInaccessibleMessage{
			Type:    models.MaybeInaccessibleMessageTypeMessage,
			Message: &models.Message{ID: 9, Chat: models.Chat{ID: 42}},
		}},
	}, "")
	if err := callback.EditRichMessage(message); err != nil {
		t.Fatalf("edit rich message: %v", err)
	}
	if len(methods) != 2 || !strings.HasSuffix(methods[0], "/sendRichMessage") || !strings.HasSuffix(methods[1], "/editMessageText") {
		t.Fatalf("Telegram methods = %v", methods)
	}
}
