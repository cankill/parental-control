package bot

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type updateContext struct {
	ctx      context.Context
	bot      *tgbot.Bot
	update   *models.Update
	payload  string
	answered bool
}

func newUpdateContext(ctx context.Context, b *tgbot.Bot, update *models.Update, payload string) *updateContext {
	return &updateContext{ctx: ctx, bot: b, update: update, payload: payload}
}

func (c *updateContext) Payload() string { return c.payload }

func (c *updateContext) Data() string { return c.payload }

func (c *updateContext) SendText(text string, parseMode models.ParseMode, markup models.ReplyMarkup) error {
	chatID, ok := c.chatID()
	if !ok {
		return fmt.Errorf("update has no accessible chat")
	}
	_, err := c.bot.SendMessage(c.ctx, &tgbot.SendMessageParams{
		ChatID: chatID, Text: text, ParseMode: parseMode, ReplyMarkup: markup,
	})
	return err
}

func (c *updateContext) ReplyText(text string, markup models.ReplyMarkup) error {
	if c.update.Message == nil {
		return fmt.Errorf("update has no message to reply to")
	}
	_, err := c.bot.SendMessage(c.ctx, &tgbot.SendMessageParams{
		ChatID:          c.update.Message.Chat.ID,
		Text:            text,
		ReplyMarkup:     markup,
		ReplyParameters: &models.ReplyParameters{MessageID: c.update.Message.ID},
	})
	return err
}

func (c *updateContext) SendRichMessage(message models.InputRichMessage) error {
	chatID, ok := c.chatID()
	if !ok {
		return fmt.Errorf("update has no accessible chat")
	}
	_, err := c.bot.SendRichMessage(c.ctx, &tgbot.SendRichMessageParams{ChatID: chatID, RichMessage: message})
	return err
}

func (c *updateContext) EditText(text string, parseMode models.ParseMode, markup models.ReplyMarkup) error {
	chatID, messageID, ok := c.callbackMessage()
	if !ok {
		return fmt.Errorf("callback has no accessible message")
	}
	_, err := c.bot.EditMessageText(c.ctx, &tgbot.EditMessageTextParams{
		ChatID: chatID, MessageID: messageID, Text: text, ParseMode: parseMode, ReplyMarkup: markup,
	})
	return err
}

func (c *updateContext) EditRichMessage(message models.InputRichMessage) error {
	chatID, messageID, ok := c.callbackMessage()
	if !ok {
		return fmt.Errorf("callback has no accessible message")
	}
	_, err := c.bot.EditMessageText(c.ctx, &tgbot.EditMessageTextParams{
		ChatID: chatID, MessageID: messageID, RichMessage: &message,
	})
	return err
}

func (c *updateContext) SendPhoto(reader io.Reader, filename, caption string, markup models.ReplyMarkup) error {
	chatID, ok := c.chatID()
	if !ok {
		return fmt.Errorf("update has no accessible chat")
	}
	_, err := c.bot.SendPhoto(c.ctx, &tgbot.SendPhotoParams{
		ChatID: chatID, Photo: &models.InputFileUpload{Filename: filename, Data: reader}, Caption: caption, ReplyMarkup: markup,
	})
	return err
}

func (c *updateContext) SendPhotoFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return c.SendPhoto(file, filepath.Base(file.Name()), "", nil)
}

func (c *updateContext) SendAudioFile(path string) error {
	chatID, ok := c.chatID()
	if !ok {
		return fmt.Errorf("update has no accessible chat")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = c.bot.SendAudio(c.ctx, &tgbot.SendAudioParams{
		ChatID: chatID, Audio: &models.InputFileUpload{Filename: filepath.Base(file.Name()), Data: file},
	})
	return err
}

func (c *updateContext) EditPhoto(reader io.Reader, filename, caption string, markup models.ReplyMarkup) error {
	chatID, messageID, ok := c.callbackMessage()
	if !ok {
		return fmt.Errorf("callback has no accessible message")
	}
	_, err := c.bot.EditMessageMedia(c.ctx, &tgbot.EditMessageMediaParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Media:       &models.InputMediaPhoto{Media: "attach://" + filename, Caption: caption, MediaAttachment: reader},
		ReplyMarkup: markup,
	})
	return err
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
