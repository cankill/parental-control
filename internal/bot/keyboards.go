package bot

import "github.com/go-telegram/bot/models"

type keyboards struct {
	youtube   *models.InlineKeyboardMarkup
	minutes30 models.InlineKeyboardButton
	hour1     models.InlineKeyboardButton
	block     models.InlineKeyboardButton
	unblock   models.InlineKeyboardButton

	stats    *models.InlineKeyboardMarkup
	hourly   models.InlineKeyboardButton
	daily    models.InlineKeyboardButton
	activity models.InlineKeyboardButton

	activityMenu   *models.InlineKeyboardMarkup
	activityHourly models.InlineKeyboardButton
	activityDaily  models.InlineKeyboardButton
	activityWeekly models.InlineKeyboardButton

	media  *models.InlineKeyboardMarkup
	photo  models.InlineKeyboardButton
	screen models.InlineKeyboardButton
	record models.InlineKeyboardButton

	web   *models.InlineKeyboardMarkup
	url   models.InlineKeyboardButton
	sites models.InlineKeyboardButton
}

func newKeyboards() *keyboards {
	k := &keyboards{}
	k.minutes30 = callbackButton("30 Minutes", "30-minutes")
	k.hour1 = callbackButton("1 Hour", "1-hour")
	k.block = callbackButton("Block", "block")
	k.unblock = callbackButton("Unblock", "un-block")
	k.youtube = inlineKeyboard(k.minutes30, k.hour1, k.block, k.unblock)

	k.hourly = callbackButton("Hourly", "hub-hourly")
	k.daily = callbackButton("Daily", "hub-daily")
	k.activity = callbackButton("Activity", "hub-activity")
	k.stats = inlineKeyboard(k.hourly, k.daily, k.activity)

	k.activityHourly = callbackButton("Hourly", "activity-hourly")
	k.activityDaily = callbackButton("Daily", "activity-daily")
	k.activityWeekly = callbackButton("Weekly", "activity-weekly")
	k.activityMenu = inlineKeyboard(k.activityHourly, k.activityDaily, k.activityWeekly)

	k.photo = callbackButton("Photo", "hub-photo")
	k.screen = callbackButton("Screen", "hub-screen")
	k.record = callbackButton("Record", "hub-record")
	k.media = inlineKeyboard(k.photo, k.screen, k.record)

	k.url = callbackButton("URL", "hub-url")
	k.sites = callbackButton("Sites", "hub-sites")
	k.web = inlineKeyboard(k.url, k.sites)
	return k
}

func callbackButton(text, action string, payload ...string) models.InlineKeyboardButton {
	return models.InlineKeyboardButton{Text: text, CallbackData: encodeCallbackData(action, payload...)}
}

func inlineKeyboard(buttons ...models.InlineKeyboardButton) *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{buttons}}
}
