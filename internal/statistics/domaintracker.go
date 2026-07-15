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
// Если frontmost-приложение не браузер (или нет разрешения Automation) — тик
// пропускается. Запускается как отдельная горутина под общим WaitGroup/ctx,
// чтобы медленный osascript (до ~3с) не блокировал обработку статистики.
func TrackDomains(ctx context.Context, interval time.Duration, commands chan<- types.AppCommand) {
	fmt.Printf("Running domain tracker (every %s)\n", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	millis := interval.Milliseconds()
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Domain tracker stopped")
			wg := ctx.Value(types.WgKey{}).(*sync.WaitGroup)
			wg.Done()
			return

		case <-ticker.C:
			url, err := browser.FrontmostBrowserURL()
			if err != nil || url == "" {
				continue // не браузер / нет разрешения / нет вкладки
			}
			domain := browser.Domain(url)
			if domain == "" {
				continue
			}
			select {
			case commands <- types.DomainTick{Domain: domain, Millis: millis}:
			case <-ctx.Done():
			}
		}
	}
}
