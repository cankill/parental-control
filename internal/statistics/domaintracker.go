package statistics

import (
	"context"
	"fmt"
	"parental-control/internal/browser"
	"parental-control/internal/lib/types"
	"sync"
	"time"
)

// TrackDomains периодически (interval) опрашивает URL активного браузера и шлёт
// в канал статистики DomainTick с временем, проведённым на домене за интервал.
// Даже если frontmost-приложение не браузер или URL пуст, наблюдение
// сохраняется: фильтрация происходит только при расчёте отчёта. Запускается
// как отдельная горутина под общим WaitGroup/ctx,
// чтобы медленный osascript (до ~3с) не блокировал обработку статистики.
func TrackDomains(ctx context.Context, interval time.Duration, commands chan<- types.AppCommand) {
	fmt.Printf("Running domain tracker (every %s)\n", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	lastPoll := time.Now()
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Domain tracker stopped")
			wg := ctx.Value(types.WgKey{}).(*sync.WaitGroup)
			wg.Done()
			return

		case <-ticker.C:
			sampledAt := time.Now()
			rawMillis := sampledAt.Sub(lastPoll).Milliseconds()
			lastPoll = sampledAt
			browserBundleID, url, err := browser.FrontmostBrowserTab()
			if err != nil && browserBundleID == "" {
				continue // even the foreground application could not be observed
			}
			domain := ""
			if err == nil && browser.IsBrowser(browserBundleID) {
				domain = browser.Domain(url)
			}
			measuredMillis := measuredDomainMillis(rawMillis, interval)
			select {
			case commands <- types.DomainTick{
				At:              sampledAt,
				BrowserBundleID: browserBundleID,
				Domain:          domain,
				RawMillis:       rawMillis,
				Millis:          measuredMillis,
			}:
			case <-ctx.Done():
			}
		}
	}
}

func measuredDomainMillis(rawMillis int64, interval time.Duration) int64 {
	intervalMillis := interval.Milliseconds()
	if rawMillis <= 0 || rawMillis > intervalMillis {
		return intervalMillis
	}
	return rawMillis
}
