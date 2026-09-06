package bot

import (
	"context"
	"fmt"
	"log"
	"parental-control/internal/lib/config"
	"parental-control/internal/lib/types"
	"strings"
	"sync"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

var defaultAdmins = []int64{183358896}

type handlerFunc func(*updateContext) error

type handlerRegistry struct {
	bot       *tgbot.Bot
	stats     *statisticsClient
	youtube   *youtubeTimer
	rich      *richClient
	commands  map[string]handlerFunc
	callbacks map[string]handlerFunc
}

func StartBot(ctx context.Context, requests chan<- types.AppCommand) {
	wg := ctx.Value(types.WgKey{}).(*sync.WaitGroup)
	defer wg.Done()

	fmt.Println("Running bot")
	env := ctx.Value(types.EnvKey{}).(*config.Env)
	admins := env.AdminIDs
	if len(admins) == 0 {
		admins = defaultAdmins
	}

	b, err := tgbot.New(
		env.BotToken,
		tgbot.WithSkipGetMe(),
		tgbot.WithAllowedUpdates(tgbot.AllowedUpdates{"message", "callback_query"}),
		tgbot.WithMiddlewares(adminOnly(admins)),
		tgbot.WithDefaultHandler(func(context.Context, *tgbot.Bot, *models.Update) {}),
	)
	if err != nil {
		log.Printf("Failed to create bot: %s", err)
		return
	}

	if _, err := b.SetMyCommands(ctx, &tgbot.SetMyCommandsParams{Commands: botCommands()}); err != nil {
		log.Printf("SetMyCommands failed: %s", err)
	}

	h := &handlerRegistry{
		bot:       b,
		stats:     newStatisticsClient(ctx, requests),
		youtube:   newYoutubeTimer(ctx),
		rich:      newRichClient(telegramAPIURL, env.BotToken, nil),
		commands:  make(map[string]handlerFunc),
		callbacks: make(map[string]handlerFunc),
	}
	h.registerStatsHandlers()
	h.registerActivityHandlers()
	h.registerWebHandlers()
	h.registerMediaHandlers()
	h.registerYoutubeHandlers()
	h.bindHandlers()

	b.Start(ctx)
	fmt.Println("Bot stopped")
}

func botCommands() []models.BotCommand {
	return []models.BotCommand{
		{Command: "stats", Description: "App usage: Hourly | Daily | Weekly"},
		{Command: "hourly", Description: "App usage this hour"},
		{Command: "daily", Description: "App usage today"},
		{Command: "weekly", Description: "App usage this week"},
		{Command: "activity", Description: "Activity: Hourly | Daily | Weekly"},
		{Command: "info", Description: "App info by name: /info <name>"},
		{Command: "web", Description: "Browser: URL | Sites"},
		{Command: "url", Description: "Current browser URL"},
		{Command: "sites", Description: "Domain usage this hour"},
		{Command: "media", Description: "Capture: Video | Screen | Record"},
		{Command: "video", Description: "5-second video from camera"},
		{Command: "screen", Description: "Screenshot"},
		{Command: "record", Description: "Record audio: /record [seconds]"},
		{Command: "presence", Description: "Check if a person is at the computer"},
		{Command: "youtube", Description: "Block / unblock YouTube"},
	}
}

func (h *handlerRegistry) command(name string, handler handlerFunc) {
	h.commands[name] = handler
}

func (h *handlerRegistry) callback(action string, handler handlerFunc) {
	h.callbacks[action] = handler
}

func (h *handlerRegistry) bindHandlers() {
	h.bot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
		if update.Message == nil {
			return false
		}
		name, _, ok := parseCommand(update.Message.Text)
		if !ok {
			return false
		}
		_, ok = h.commands[name]
		return ok
	}, h.dispatchCommand)

	h.bot.RegisterHandlerMatchFunc(func(update *models.Update) bool {
		return update.CallbackQuery != nil
	}, h.dispatchCallback)
}

func (h *handlerRegistry) dispatchCommand(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	name, payload, ok := parseCommand(update.Message.Text)
	if !ok {
		return
	}
	handler, ok := h.commands[name]
	if !ok {
		return
	}
	c := newUpdateContext(ctx, b, update, payload, h.rich)
	if err := handler(c); err != nil {
		log.Printf("Telegram command /%s failed: %s", name, err)
	}
}

func (h *handlerRegistry) dispatchCallback(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	c := newUpdateContext(ctx, b, update, "", h.rich)
	defer c.ensureCallbackAnswered()
	action, payload, ok := parseCallbackData(update.CallbackQuery.Data)
	if !ok {
		return
	}
	handler, ok := h.callbacks[action]
	if !ok {
		return
	}
	c.payload = payload
	if err := handler(c); err != nil {
		log.Printf("Telegram callback %s failed: %s", action, err)
	}
}

func (h *handlerRegistry) hubAction(action handlerFunc) handlerFunc {
	return func(c *updateContext) error {
		if err := c.AnswerCallback("", false); err != nil {
			log.Printf("Answer hub callback failed: %s", err)
		}
		return action(c)
	}
}

func parseCommand(text string) (name, payload string, ok bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return "", "", false
	}
	token := text
	if i := strings.IndexAny(text, " \t\r\n"); i >= 0 {
		token = text[:i]
		payload = strings.TrimSpace(text[i:])
	}
	name = strings.TrimPrefix(token, "/")
	if i := strings.IndexByte(name, '@'); i >= 0 {
		name = name[:i]
	}
	if name == "" {
		return "", "", false
	}
	return strings.ToLower(name), payload, true
}

func encodeCallbackData(action string, payload ...string) string {
	data := "\f" + action
	if len(payload) > 0 && payload[0] != "" {
		data += "|" + payload[0]
	}
	return data
}

func parseCallbackData(data string) (action, payload string, ok bool) {
	if !strings.HasPrefix(data, "\f") {
		return "", "", false
	}
	data = strings.TrimPrefix(data, "\f")
	action, payload, _ = strings.Cut(data, "|")
	if action == "" {
		return "", "", false
	}
	return action, payload, true
}

func adminOnly(admins []int64) tgbot.Middleware {
	allowed := make(map[int64]struct{}, len(admins))
	for _, id := range admins {
		allowed[id] = struct{}{}
	}
	return func(next tgbot.HandlerFunc) tgbot.HandlerFunc {
		return func(ctx context.Context, b *tgbot.Bot, update *models.Update) {
			id, ok := updateSenderID(update)
			if !ok {
				return
			}
			if _, ok := allowed[id]; !ok {
				return
			}
			next(ctx, b, update)
		}
	}
}

func updateSenderID(update *models.Update) (int64, bool) {
	if update == nil {
		return 0, false
	}
	if update.Message != nil && update.Message.From != nil {
		return update.Message.From.ID, true
	}
	if update.CallbackQuery != nil {
		return update.CallbackQuery.From.ID, true
	}
	return 0, false
}
