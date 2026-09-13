package presence

import (
	"testing"
	"time"

	"parental-control/internal/lib/types"
)

func TestMachineDebouncesAbsenceAndReportsReturn(t *testing.T) {
	machine := newMachine(3)
	start := time.Date(2026, 9, 7, 9, 0, 0, 0, time.Local)
	if event := machine.observe(start, ObservationPresent); event != nil {
		t.Fatalf("initial presence event = %+v", event)
	}
	if event := machine.observe(start.Add(time.Minute), ObservationMissing); event != nil {
		t.Fatalf("first miss event = %+v", event)
	}
	if event := machine.observe(start.Add(2*time.Minute), ObservationMissing); event != nil {
		t.Fatalf("second miss event = %+v", event)
	}
	absent := machine.observe(start.Add(3*time.Minute), ObservationMissing)
	if absent == nil || absent.Kind != EventAbsent || !absent.Since.Equal(start.Add(time.Minute)) {
		t.Fatalf("absence event = %+v", absent)
	}
	if event := machine.observe(start.Add(4*time.Minute), ObservationMissing); event != nil {
		t.Fatalf("repeated absence event = %+v", event)
	}
	returned := machine.observe(start.Add(5*time.Minute), ObservationPresent)
	if returned == nil || returned.Kind != EventReturned || !returned.Since.Equal(start.Add(time.Minute)) {
		t.Fatalf("return event = %+v", returned)
	}
}

func TestObservationSampleStoresOnlyPrivacyPreservingState(t *testing.T) {
	at := time.Date(2026, 9, 7, 9, 0, 0, 0, time.Local)
	sample, ok := observationSample(at, time.Minute, 3, ObservationMissing)
	if !ok || sample.At != at || sample.Kind != types.PresenceMissing || sample.Seconds != 60 || sample.MissThreshold != 3 {
		t.Fatalf("presence sample = %+v, %v", sample, ok)
	}
	if _, ok := observationSample(at, time.Minute, 3, ObservationOutsideSchedule); ok {
		t.Fatal("outside-schedule observation was persisted")
	}
}

func TestMachineNeverTreatsCameraErrorsAsAbsence(t *testing.T) {
	machine := newMachine(3)
	start := time.Date(2026, 9, 7, 9, 0, 0, 0, time.Local)
	for i := 0; i < 2; i++ {
		if event := machine.observe(start.Add(time.Duration(i)*time.Minute), ObservationUnavailable); event != nil {
			t.Fatalf("early unavailable event = %+v", event)
		}
	}
	event := machine.observe(start.Add(2*time.Minute), ObservationUnavailable)
	if event == nil || event.Kind != EventUnavailable || event.State == StateAbsent {
		t.Fatalf("unavailable event = %+v", event)
	}
	if repeated := machine.observe(start.Add(3*time.Minute), ObservationUnavailable); repeated != nil {
		t.Fatalf("repeated unavailable event = %+v", repeated)
	}
}

func TestScheduleUsesLocalWeekdaysAndHours(t *testing.T) {
	options := OptionsFromValues(true, 60, 120, 3, 8, 18, []int{1, 2, 3, 4, 5})
	policy := defaultPolicy(options)
	for _, test := range []struct {
		at   time.Time
		want bool
	}{
		{time.Date(2026, 9, 7, 8, 0, 0, 0, time.Local), true}, // Monday start
		{time.Date(2026, 9, 7, 17, 59, 0, 0, time.Local), true},
		{time.Date(2026, 9, 7, 18, 0, 0, 0, time.Local), false},
		{time.Date(2026, 9, 6, 12, 0, 0, 0, time.Local), false}, // Sunday
	} {
		if got := policyActiveAt(policy, test.at); got != test.want {
			t.Errorf("policyActiveAt(%v) = %v, want %v", test.at, got, test.want)
		}
	}
}

func TestOutsideScheduleClearsPendingAbsence(t *testing.T) {
	machine := newMachine(2)
	start := time.Date(2026, 9, 7, 17, 59, 0, 0, time.Local)
	_ = machine.observe(start, ObservationMissing)
	_ = machine.observe(start.Add(time.Minute), ObservationOutsideSchedule)
	if event := machine.observe(start.Add(24*time.Hour), ObservationMissing); event != nil {
		t.Fatalf("miss leaked across schedule boundary: %+v", event)
	}
}

func TestSignalReportsOnlyConfirmedPresence(t *testing.T) {
	signal := NewSignal()
	if signal.Present() {
		t.Fatal("new signal unexpectedly reports presence")
	}
	at := time.Date(2026, 9, 13, 9, 0, 0, 0, time.Local)
	signal.update(StatePresent, at)
	if !signal.Present() {
		t.Fatal("confirmed presence was not exposed")
	}
	state, updated := signal.Snapshot()
	if state != StatePresent || !updated.Equal(at) {
		t.Fatalf("signal snapshot = %s %s", state, updated)
	}
	signal.update(StateSuspectedAbsent, at.Add(time.Minute))
	if signal.Present() {
		t.Fatal("suspected absence still exposed as presence")
	}
}
