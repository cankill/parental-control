package statstorage

import (
	"parental-control/internal/lib/storage/local/diskvstorage"
	"strings"
	"testing"
	"time"
)

func newTestStorage(t *testing.T) *StatsStorage {
	t.Helper()
	return &StatsStorage{localStorage: diskvstorage.OpenStorage(t.TempDir())}
}

func TestDisplayName(t *testing.T) {
	cases := map[string]string{
		"com.spotify.client": "Client",
		"com.google.Chrome":  "Chrome",
		"com.apple.Terminal": "Terminal",
		"single":             "Single",
	}
	for in, want := range cases {
		if got := DisplayName(in); got != want {
			t.Errorf("DisplayName(%q) = %q, want %q", in, got, want)
		}
	}
}

// FindAppInfoByName ищет по отображаемому имени (без учёта регистра) и возвращает
// все совпадения. Сохраняем метаданные напрямую в store (минуя mdfind-резолв).
func TestFindAppInfoByName(t *testing.T) {
	st := newTestStorage(t)
	st.localStorage.SaveValue(appInfoBucket, "com.spotify.client",
		`{"bundle_id":"com.spotify.client","path":"/Applications/Spotify.app","name":"Spotify","version":"1.2"}`)
	st.localStorage.SaveValue(appInfoBucket, "com.google.Chrome",
		`{"bundle_id":"com.google.Chrome","path":"/Applications/Google Chrome.app"}`)

	got := st.FindAppInfoByName("client") // регистр не важен
	if len(got) != 1 || got[0].BundleID != "com.spotify.client" || got[0].Name != "Spotify" {
		t.Fatalf("FindAppInfoByName(client) = %+v", got)
	}
	if len(st.FindAppInfoByName("nonexistent")) != 0 {
		t.Error("expected no matches for nonexistent name")
	}
	// Домены/статистика не должны попасть в словарь приложений.
	st.AddDomainTime("youtube.com", 1000)
	if len(st.FindAppInfoByName("com")) != 0 {
		t.Error("domain leaked into app dictionary search")
	}
}

// IncreaseStatistics в пределах одного часа записывает всё время в текущий bucket.
func TestIncreaseStatisticsSingleHour(t *testing.T) {
	st := newTestStorage(t)
	from := time.Now().Add(-2 * time.Minute)
	st.IncreaseStatistics("com.google.Chrome", from)

	resp := st.GetStatisticsShifted(0)
	if len(resp.AppInfos) != 1 || resp.AppInfos[0].Identity != "Chrome" {
		t.Fatalf("expected Chrome in current hour: %+v", resp.AppInfos)
	}
	// Примерно 2 минуты (допускаем небольшой дрейф на время выполнения теста).
	d := resp.AppInfos[0].Duration
	if d < 90*time.Second || d > 3*time.Minute {
		t.Errorf("duration %v not ~2m", d)
	}
}

func TestLoginWindowIsTreatedAsIdle(t *testing.T) {
	st := newTestStorage(t)
	from := time.Now().Add(-2 * time.Minute)
	st.IncreaseStatistics("com.apple.loginwindow", from)

	if got := st.GetStatisticsShifted(0).AppInfos; len(got) != 0 {
		t.Fatalf("loginwindow time was recorded as activity: %+v", got)
	}
}

func TestStatisticsHideStoredLoginWindowTime(t *testing.T) {
	st := newTestStorage(t)
	bucket := time.Now().Format(TruncatedToHour)
	st.localStorage.SaveValue(bucket, "com.google.Chrome", "60000")
	st.localStorage.SaveValue(bucket, "com.apple.loginwindow", "300000")

	stats := st.GetStatistics(bucket)
	if len(stats) != 1 || stats[0].Identity != "Chrome" || stats[0].Duration != time.Minute {
		t.Fatalf("filtered statistics = %+v, want only Chrome for 1m", stats)
	}

	table := stats.FormatTable()
	if strings.Contains(strings.ToLower(table), "loginwindow") {
		t.Fatalf("loginwindow leaked into rendered table:\n%s", table)
	}
	if !strings.Contains(table, "1m0s") || strings.Contains(table, "6m0s") {
		t.Fatalf("rendered total includes idle time:\n%s", table)
	}
}

// IncreaseStatistics, пересекающая границу часа, раскладывает время по двум
// bucket'ам (регрессия целочисленной арифметики часов вместо float64).
func TestIncreaseStatisticsCrossesHourBoundary(t *testing.T) {
	st := newTestStorage(t)
	// Начали 90 минут назад → время попадёт минимум в 2 разных часовых bucket'а.
	from := time.Now().Add(-90 * time.Minute)
	st.IncreaseStatistics("com.apple.Terminal", from)

	// Суммарно за 3 последних часа должно набраться ~90 минут Terminal.
	var total time.Duration
	for shift := 0; shift <= 2; shift++ {
		for _, ai := range st.GetStatisticsShifted(shift).AppInfos {
			if ai.Identity == "Terminal" {
				total += ai.Duration
			}
		}
	}
	if total < 80*time.Minute || total > 100*time.Minute {
		t.Errorf("total across hours = %v, want ~90m", total)
	}
}

// GetStatisticsShifted для часа без данных возвращает пустой набор, не паникуя.
func TestGetStatisticsShiftedEmpty(t *testing.T) {
	st := newTestStorage(t)
	resp := st.GetStatisticsShifted(5)
	if len(resp.AppInfos) != 0 {
		t.Errorf("empty hour should have no apps: %+v", resp.AppInfos)
	}
	if resp.TimeStamp == "" {
		t.Error("TimeStamp should still be set for empty hour")
	}
}

// Доменная статистика: AddDomainTime аккумулирует, GetDomainStatistics читает,
// имя домена не дробится по точкам, и домены не смешиваются с приложениями.
func TestDomainStatistics(t *testing.T) {
	st := newTestStorage(t)

	st.AddDomainTime("youtube.com", 30000)
	st.AddDomainTime("youtube.com", 15000) // накопление → 45s
	st.AddDomainTime("github.com", 20000)
	st.AddDomainTime("", 5000)             // пустой домен игнорируется
	st.AddDomainTime("example.com", -1000) // не положительное время игнорируется

	resp := st.GetDomainStatistics(0)
	got := map[string]time.Duration{}
	for _, ai := range resp.AppInfos {
		got[ai.Identity] = ai.Duration
	}
	if got["youtube.com"] != 45*time.Second {
		t.Fatalf("youtube.com: got %v want 45s", got["youtube.com"])
	}
	if got["github.com"] != 20*time.Second {
		t.Fatalf("github.com: got %v want 20s", got["github.com"])
	}
	if len(resp.AppInfos) != 2 {
		t.Fatalf("expected 2 domains, got %d: %+v", len(resp.AppInfos), resp.AppInfos)
	}
	// Домены не попадают в статистику приложений (разные bucket-префиксы).
	if len(st.GetStatisticsShifted(0).AppInfos) != 0 {
		t.Fatalf("app stats should be empty, domains leaked in")
	}
}

// NearestShift должен перепрыгивать пропущенные часы к ближайшему непустому bucket'у.
func TestNearestShiftSkipsGaps(t *testing.T) {
	st := newTestStorage(t)
	now := time.Now().Truncate(time.Hour)
	for _, shift := range []int{0, 3, 5} { // дыры на 1,2,4
		bucket := now.Add(-time.Duration(shift) * time.Hour).Format(TruncatedToHour)
		st.localStorage.SaveValue(bucket, "com.test.app", "1000")
	}
	// Час только с loginwindow считается пустым и также перепрыгивается.
	st.localStorage.SaveValue(now.Add(-time.Hour).Format(TruncatedToHour), "com.apple.loginwindow", "3600000")

	if s, ok := st.NearestShift(0, true); !ok || s != 3 {
		t.Fatalf("older from 0: got (%d,%v), want (3,true)", s, ok)
	}
	if s, ok := st.NearestShift(3, true); !ok || s != 5 {
		t.Fatalf("older from 3: got (%d,%v), want (5,true)", s, ok)
	}
	if _, ok := st.NearestShift(5, true); ok {
		t.Fatalf("older from 5: expected none")
	}
	if s, ok := st.NearestShift(5, false); !ok || s != 3 {
		t.Fatalf("newer from 5: got (%d,%v), want (3,true)", s, ok)
	}
	if _, ok := st.NearestShift(0, false); ok {
		t.Fatalf("newer from 0: expected none")
	}
}

// GetStatisticsDay суммирует время приложения по всем часам календарного дня,
// NearestDayShift перепрыгивает пустые дни.
func TestGetStatisticsDayAndNav(t *testing.T) {
	st := newTestStorage(t)
	now := time.Now()

	today := now.Format(TruncatedToDay)
	st.localStorage.SaveValue(today+"T10", "com.google.Chrome", "60000")
	st.localStorage.SaveValue(today+"T14", "com.google.Chrome", "30000")
	// Вчера было только окно входа: для статистики активности день пустой.
	yesterday := now.AddDate(0, 0, -1).Format(TruncatedToDay)
	st.localStorage.SaveValue(yesterday+"T12", "com.apple.loginwindow", "3600000")
	d2 := now.AddDate(0, 0, -2).Format(TruncatedToDay)
	st.localStorage.SaveValue(d2+"T09", "com.apple.Safari", "45000")
	// Старое накопленное время loginwindow за сегодня не входит в дневной total.
	st.localStorage.SaveValue(today+"T14", "com.apple.loginwindow", "300000")

	resp := st.GetStatisticsDay(0)
	if resp.TimeStamp != today {
		t.Fatalf("day timestamp: got %s want %s", resp.TimeStamp, today)
	}
	if len(resp.AppInfos) != 1 || resp.AppInfos[0].Identity != "Chrome" || resp.AppInfos[0].Duration != 90*time.Second {
		t.Fatalf("today agg wrong: %+v", resp.AppInfos)
	}

	if s, ok := st.NearestDayShift(0, true); !ok || s != 2 {
		t.Fatalf("older day from 0: got (%d,%v) want (2,true)", s, ok)
	}
	if s, ok := st.NearestDayShift(2, false); !ok || s != 0 {
		t.Fatalf("newer day from 2: got (%d,%v) want (0,true)", s, ok)
	}
	if _, ok := st.NearestDayShift(2, true); ok {
		t.Fatalf("older day from 2: expected none")
	}
}

func TestGetStatisticsWeekAndNav(t *testing.T) {
	st := newTestStorage(t)
	now := time.Now()
	currentWeek := activityWeekStart(now)
	previousWeek := currentWeek.AddDate(0, 0, -7)

	st.localStorage.SaveValue(currentWeek.Format(TruncatedToDay)+"T10", "com.google.Chrome", "60000")
	st.localStorage.SaveValue(currentWeek.AddDate(0, 0, 2).Format(TruncatedToDay)+"T14", "com.google.Chrome", "30000")
	st.localStorage.SaveValue(currentWeek.AddDate(0, 0, 3).Format(TruncatedToDay)+"T11", "com.apple.Safari", "45000")
	st.localStorage.SaveValue(previousWeek.Format(TruncatedToDay)+"T09", "com.apple.Safari", "120000")
	st.localStorage.SaveValue(previousWeek.AddDate(0, 0, -7).Format(TruncatedToDay)+"T09", "com.apple.loginwindow", "3600000")

	resp := st.GetStatisticsWeek(0)
	wantTimestamp := currentWeek.Format(TruncatedToDay) + " – " + currentWeek.AddDate(0, 0, 6).Format(TruncatedToDay)
	if resp.TimeStamp != wantTimestamp {
		t.Fatalf("week timestamp = %q, want %q", resp.TimeStamp, wantTimestamp)
	}
	if len(resp.AppInfos) != 2 || resp.AppInfos[0].Identity != "Chrome" || resp.AppInfos[0].Duration != 90*time.Second {
		t.Fatalf("current week aggregation = %+v", resp.AppInfos)
	}
	if shift, ok := st.NearestWeekShift(0, true); !ok || shift != 1 {
		t.Fatalf("older week from 0 = (%d, %v), want (1, true)", shift, ok)
	}
	if shift, ok := st.NearestWeekShift(1, false); !ok || shift != 0 {
		t.Fatalf("newer week from 1 = (%d, %v), want (0, true)", shift, ok)
	}
	if _, ok := st.NearestWeekShift(1, true); ok {
		t.Fatal("loginwindow-only week must not be available for navigation")
	}
}
