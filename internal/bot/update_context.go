package bot

import (
	"context"
	"fmt"
	"log"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type updateContext struct {
	ctx      context.Context
	bot      *tgbot.Bot
	update   *models.Update
	payload  string
	answered bool
	rich     *richClient
}

func newUpdateContext(ctx context.Context, b *tgbot.Bot, update *models.Update, payload string, clients ...*richClient) *updateContext {
	client := newRichClient(telegramAPIURL, b.Token(), nil)
	if len(clients) > 0 && clients[0] != nil {
		client = clients[0]
	}
	return &updateContext{ctx: ctx, bot: b, update: update, payload: payload, rich: client}
}

func (c *updateContext) Payload() string { return c.payload }

func (c *updateContext) Data() string { return c.payload }

func (c *updateContext) SendRichMessage(message models.InputRichMessage) error {
	chatID, ok := c.chatID()
	if !ok {
		return fmt.Errorf("update has no accessible chat")
	}
	return c.rich.send(c.ctx, chatID, message, 0)
}

func (c *updateContext) ReplyRichMessage(message models.InputRichMessage) error {
	if c.update.Message == nil {
		return fmt.Errorf("update has no message to reply to")
	}
	return c.rich.send(c.ctx, c.update.Message.Chat.ID, message, c.update.Message.ID)
}

func (c *updateContext) EditRichMessage(message models.InputRichMessage) error {
	chatID, messageID, ok := c.callbackMessage()
	if !ok {
		return fmt.Errorf("callback has no accessible message")
	}
	return c.rich.edit(c.ctx, chatID, messageID, message)
}

func (c *updateContext) AnswerCallback(text string, showAlert bool) error {
	if c.update.CallbackQuery == nil {
		return fmt.Errorf("update is not a callback query")
	}
	c.answered = true
	_, err := c.bot.AnswerCallbackQuery(c.ctx, &tgbot.AnswerCallbackQueryParams{
		CallbackQueryID: c.update.CallbackQuery.ID, Text: text, ShowAlert: showAlert,
	})
	return err
}

func (c *updateContext) ensureCallbackAnswered() {
	if c.update.CallbackQuery == nil || c.answered {
		return
	}
	if err := c.AnswerCallback("", false); err != nil {
		log.Printf("Answer callback failed: %s", err)
	}
}

func (c *updateContext) chatID() (int64, bool) {
	if c.update.Message != nil {
		return c.update.Message.Chat.ID, true
	}
	chatID, _, ok := c.callbackMessage()
	return chatID, ok
}

func (c *updateContext) callbackMessage() (int64, int, bool) {
	if c.update.CallbackQuery == nil {
		return 0, 0, false
	}
	message := c.update.CallbackQuery.Message
	switch message.Type {
	case models.MaybeInaccessibleMessageTypeMessage:
		if message.Message != nil {
			return message.Message.Chat.ID, message.Message.ID, true
		}
	case models.MaybeInaccessibleMessageTypeInaccessibleMessage:
		if message.InaccessibleMessage != nil {
			return message.InaccessibleMessage.Chat.ID, message.InaccessibleMessage.MessageID, true
		}
	}
	return 0, 0, false
}
