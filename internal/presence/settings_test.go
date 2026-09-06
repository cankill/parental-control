package presence

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func settingsTestNow() time.Time {
	return time.Date(2026, 9, 7, 16, 30, 15, 0, time.Local)
}

func TestControllerSupportsConcurrentBotUpdatesAndMonitorReads(t *testing.T) {
	options := OptionsFromValues(true, 60, 120, 3, 8, 18, []int{1, 2, 3, 4, 5})
	controller, err := OpenController(filepath.Join(t.TempDir(), "presence.json"), options)
	if err != nil {
		t.Fatal(err)
	}
	now := settingsTestNow()
	var wait sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			for iteration := 0; iteration < 20; iteration++ {
				if worker%2 == 0 {
					_, _ = controller.Enable("08:00-18:00 Mon,Tue,Fri", now)
				} else {
					_ = controller.Snapshot(now)
				}
			}
		}(worker)
	}
	wait.Wait()
}

func TestEnableWithoutParametersRemovesAllRestrictions(t *testing.T) {
	current := Policy{Enabled: false, Mode: ScheduleDaily, StartMinute: 8 * 60, EndMinute: 18 * 60, WorkDays: []int{1, 2, 3}}
	updated, err := applyExpression(current, "", settingsTestNow())
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Enabled || updated.Mode != ScheduleAlways || !updated.Until.IsZero() || len(updated.WorkDays) != 0 {
		t.Fatalf("unrestricted policy = %+v", updated)
	}
}

func TestClockEnablesUntilNextOccurrence(t *testing.T) {
	now := settingsTestNow()
	for _, test := range []struct {
		expression string
		want       time.Time
	}{
		{"18:45", time.Date(2026, 9, 7, 18, 45, 0, 0, time.Local)},
		{"15:00", time.Date(2026, 9, 8, 15, 0, 0, 0, time.Local)},
	} {
		updated, err := applyExpression(Policy{Mode: ScheduleAlways}, test.expression, now)
		if err != nil {
			t.Fatal(err)
		}
		if updated.Mode != ScheduleUntil || !updated.Until.Equal(test.want) {
			t.Errorf("%s policy = %+v, want until %v", test.expression, updated, test.want)
		}
	}
}

func TestExtendedDurationSupportsDaysHoursMinutesAndSeconds(t *testing.T) {
	now := settingsTestNow()
	updated, err := applyExpression(Policy{Mode: ScheduleAlways}, "1d10h20m30s", now)
	if err != nil {
		t.Fatal(err)
	}
	want := now.Add(24*time.Hour + 10*time.Hour + 20*time.Minute + 30*time.Second)
	if updated.Mode != ScheduleUntil || !updated.Until.Equal(want) {
		t.Fatalf("duration policy = %+v, want %v", updated, want)
	}
	for _, invalid := range []string{"0s", "10", "1h2d", "1.5h"} {
		if _, err := applyExpression(Policy{Mode: ScheduleAlways}, invalid, now); err == nil {
			t.Errorf("invalid duration %q was accepted", invalid)
		}
	}
}

func TestDailyWindowAndWeekdaysCanBeCombinedOrUpdatedSeparately(t *testing.T) {
	now := settingsTestNow()
	combined, err := applyExpression(Policy{Mode: ScheduleAlways}, "08:15-18:45 Mon,Tue,Fri", now)
	if err != nil {
		t.Fatal(err)
	}
	if combined.Mode != ScheduleDaily || combined.StartMinute != 8*60+15 || combined.EndMinute != 18*60+45 {
		t.Fatalf("daily policy = %+v", combined)
	}
	if len(combined.WorkDays) != 3 || combined.WorkDays[0] != 1 || combined.WorkDays[2] != 5 {
		t.Fatalf("daily workdays = %v", combined.WorkDays)
	}

	daysOnly, err := applyExpression(combined, "Wed,Sat", now)
	if err != nil {
		t.Fatal(err)
	}
	if daysOnly.Mode != ScheduleDaily || daysOnly.StartMinute != combined.StartMinute || daysOnly.EndMinute != combined.EndMinute {
		t.Fatalf("weekday update lost time settings: %+v", daysOnly)
	}
	if len(daysOnly.WorkDays) != 2 || daysOnly.WorkDays[0] != 3 || daysOnly.WorkDays[1] != 6 {
		t.Fatalf("updated workdays = %v", daysOnly.WorkDays)
	}
}

func TestOvernightWindowUsesStartDay(t *testing.T) {
	policy := Policy{Enabled: true, Mode: ScheduleDaily, StartMinute: 22 * 60, EndMinute: 6 * 60, WorkDays: []int{1}}
	for _, test := range []struct {
		at   time.Time
		want bool
	}{
		{time.Date(2026, 9, 7, 23, 0, 0, 0, time.Local), true},  // Monday
		{time.Date(2026, 9, 8, 2, 0, 0, 0, time.Local), true},   // Tuesday, Monday's window
		{time.Date(2026, 9, 8, 23, 0, 0, 0, time.Local), false}, // Tuesday's window is disabled
	} {
		if got := policyActiveAt(policy, test.at); got != test.want {
			t.Errorf("policyActiveAt(%v) = %v, want %v", test.at, got, test.want)
		}
	}
}

func TestControllerPersistsAndExpiresSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "presence.json")
	options := OptionsFromValues(true, 60, 120, 3, 8, 18, []int{1, 2, 3, 4, 5})
	controller, err := OpenController(path, options)
	if err != nil {
		t.Fatal(err)
	}
	now := settingsTestNow()
	if _, err := controller.Enable("30s", now); err != nil {
		t.Fatal(err)
	}
	reloaded, err := OpenController(path, options)
	if err != nil {
		t.Fatal(err)
	}
	before := reloaded.Snapshot(now.Add(29 * time.Second))
	if !before.Policy.Enabled || !before.ActiveNow {
		t.Fatalf("reloaded snapshot before expiry = %+v", before)
	}
	after := reloaded.Snapshot(now.Add(30 * time.Second))
	if after.Policy.Enabled || after.ActiveNow {
		t.Fatalf("snapshot after expiry = %+v", after)
	}
	again, err := OpenController(path, options)
	if err != nil {
		t.Fatal(err)
	}
	if again.Snapshot(now.Add(time.Minute)).Policy.Enabled {
		t.Fatal("expired disabled state was not persisted")
	}
}
