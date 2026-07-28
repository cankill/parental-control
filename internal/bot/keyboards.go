package bot

import tele "gopkg.in/telebot.v4"

type keyboards struct {
	youtube   *tele.ReplyMarkup
	minutes30 tele.Btn
	hour1     tele.Btn
	block     tele.Btn
	unblock   tele.Btn

	stats    *tele.ReplyMarkup
	hourly   tele.Btn
	daily    tele.Btn
	activity tele.Btn

	activityMenu   *tele.ReplyMarkup
	activityHourly tele.Btn
	activityDaily  tele.Btn
	activityWeekly tele.Btn

	media  *tele.ReplyMarkup
	photo  tele.Btn
	screen tele.Btn
	record tele.Btn

	web   *tele.ReplyMarkup
	url   tele.Btn
	sites tele.Btn

	statPrev     tele.Btn
	statNext     tele.Btn
	dayPrev      tele.Btn
	dayNext      tele.Btn
	sitesPrev    tele.Btn
	sitesNext    tele.Btn
	activityPrev tele.Btn
	activityNext tele.Btn
}

func newKeyboards() *keyboards {
	k := &keyboards{
		youtube:      &tele.ReplyMarkup{},
		stats:        &tele.ReplyMarkup{},
		activityMenu: &tele.ReplyMarkup{},
		media:        &tele.ReplyMarkup{},
		web:          &tele.ReplyMarkup{},
	}
	k.minutes30 = k.youtube.Data("30 Minutes", "30-minutes")
	k.hour1 = k.youtube.Data("1 Hour", "1-hour")
	k.block = k.youtube.Data("Block", "block")
	k.unblock = k.youtube.Data("Unblock", "un-block")
	k.youtube.Inline(k.youtube.Row(k.minutes30, k.hour1, k.block, k.unblock))

	k.hourly = k.stats.Data("Hourly", "hub-hourly")
	k.daily = k.stats.Data("Daily", "hub-daily")
	k.activity = k.stats.Data("Activity", "hub-activity")
	k.stats.Inline(k.stats.Row(k.hourly, k.daily, k.activity))

	k.activityHourly = k.activityMenu.Data("Hourly", "activity-hourly")
	k.activityDaily = k.activityMenu.Data("Daily", "activity-daily")
	k.activityWeekly = k.activityMenu.Data("Weekly", "activity-weekly")
	k.activityMenu.Inline(k.activityMenu.Row(k.activityHourly, k.activityDaily, k.activityWeekly))

	k.photo = k.media.Data("Photo", "hub-photo")
	k.screen = k.media.Data("Screen", "hub-screen")
	k.record = k.media.Data("Record", "hub-record")
	k.media.Inline(k.media.Row(k.photo, k.screen, k.record))

	k.url = k.web.Data("URL", "hub-url")
	k.sites = k.web.Data("Sites", "hub-sites")
	k.web.Inline(k.web.Row(k.url, k.sites))

	nav := &tele.ReplyMarkup{}
	k.statPrev = nav.Data("‹ Earlier", "stat-prev")
	k.statNext = nav.Data("Later ›", "stat-next")
	k.dayPrev = nav.Data("‹ Prev day", "day-prev")
	k.dayNext = nav.Data("Next day ›", "day-next")
	k.sitesPrev = nav.Data("‹ Earlier", "sites-prev")
	k.sitesNext = nav.Data("Later ›", "sites-next")
	k.activityPrev = nav.Data("‹ Earlier", "activity-prev")
	k.activityNext = nav.Data("Later ›", "activity-next")
	return k
}
