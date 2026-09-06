// Package presence provides a privacy-preserving workstation presence monitor.
// Recent HID activity is preferred over camera checks; transient camera frames
// are analyzed locally and deleted immediately.
package presence

import (
	"context"
	"log"
	"os"
	"time"

	"parental-control/internal/activity"
	"parental-control/internal/media"
	"parental-control/internal/vision"
)

type State string

const (
	StateUnknown         State = "unknown"
	StatePresent         State = "present"
	StateSuspectedAbsent State = "suspected_absent"
	StateAbsent          State = "absent"
)

type Observation int

const (
	ObservationOutsideSchedule Observation = iota
	ObservationPresent
	ObservationMissing
	ObservationUnavailable
)

type EventKind string

const (
	EventAbsent      EventKind = "absent"
	EventReturned    EventKind = "returned"
	EventUnavailable EventKind = "unavailable"
)

type Event struct {
	Kind  EventKind
	At    time.Time
	Since time.Time
	State State
}

type Options struct {
	Enabled       bool
	Interval      time.Duration
	IdleGrace     time.Duration
	MissThreshold int
	StartHour     int
	EndHour       int
	WorkDays      []int
}

func (o Options) normalized() Options {
	if o.Interval < 10*time.Second {
		o.Interval = time.Minute
	}
	if o.IdleGrace < o.Interval {
		o.IdleGrace = 2 * o.Interval
	}
	if o.MissThreshold < 1 {
		o.MissThreshold = 3
	}
	if o.StartHour < 0 || o.StartHour > 23 {
		o.StartHour = 8
	}
	if o.EndHour < 1 || o.EndHour > 24 {
		o.EndHour = 18
	}
	if len(o.WorkDays) == 0 {
		o.WorkDays = []int{1, 2, 3, 4, 5}
	}
	return o
}

func OptionsFromValues(enabled bool, intervalSeconds, idleGraceSeconds, missThreshold, startHour, endHour int, workDays []int) Options {
	return (Options{
		Enabled:       enabled,
		Interval:      time.Duration(intervalSeconds) * time.Second,
		IdleGrace:     time.Duration(idleGraceSeconds) * time.Second,
		MissThreshold: missThreshold,
		StartHour:     startHour,
		EndHour:       endHour,
		WorkDays:      append([]int(nil), workDays...),
	}).normalized()
}

type machine struct {
	state               State
	missThreshold       int
	misses              int
	firstMissingAt      time.Time
	failures            int
	unavailableNotified bool
	absentSince         time.Time
}

func newMachine(missThreshold int) *machine {
	return &machine{state: StateUnknown, missThreshold: missThreshold}
}

func (m *machine) observe(at time.Time, observation Observation) *Event {
	switch observation {
	case ObservationOutsideSchedule:
		m.state = StateUnknown
		m.misses = 0
		m.failures = 0
		m.firstMissingAt = time.Time{}
		m.absentSince = time.Time{}
		m.unavailableNotified = false
		return nil
	case ObservationUnavailable:
		m.misses = 0
		m.firstMissingAt = time.Time{}
		m.failures++
		if m.failures >= m.missThreshold && !m.unavailableNotified {
			m.unavailableNotified = true
			return &Event{Kind: EventUnavailable, At: at, State: m.state}
		}
		return nil
	case ObservationPresent:
		wasAbsent := m.state == StateAbsent
		absentSince := m.absentSince
		m.state = StatePresent
		m.misses = 0
		m.failures = 0
		m.firstMissingAt = time.Time{}
		m.absentSince = time.Time{}
		m.unavailableNotified = false
		if wasAbsent {
			return &Event{Kind: EventReturned, At: at, Since: absentSince, State: m.state}
		}
		return nil
	case ObservationMissing:
		m.failures = 0
		m.unavailableNotified = false
		if m.state == StateAbsent {
			return nil
		}
		if m.misses == 0 {
			m.firstMissingAt = at
		}
		m.misses++
		if m.misses < m.missThreshold {
			m.state = StateSuspectedAbsent
			return nil
		}
		m.state = StateAbsent
		m.absentSince = m.firstMissingAt
		return &Event{Kind: EventAbsent, At: at, Since: m.absentSince, State: m.state}
	default:
		return nil
	}
}

func withinSchedule(at time.Time, options Options) bool {
	weekdayAllowed := false
	for _, day := range options.WorkDays {
		if int(at.Weekday()) == day {
			weekdayAllowed = true
			break
		}
	}
	if !weekdayAllowed {
		return false
	}
	if options.StartHour == options.EndHour {
		return true
	}
	return at.Hour() >= options.StartHour && at.Hour() < options.EndHour
}

func localCameraObservation() (Observation, error) {
	path, err := media.CaptureAnalysisFrame()
	if err != nil {
		return ObservationUnavailable, err
	}
	defer os.Remove(path)
	detection, err := vision.AnalyzeImage(path)
	if err != nil {
		return ObservationUnavailable, err
	}
	if detection.Present() {
		return ObservationPresent, nil
	}
	return ObservationMissing, nil
}

// Monitor observes presence until ctx is cancelled. It emits only state
// transitions, so a prolonged absence never creates repeated alerts.
func Monitor(ctx context.Context, options Options, input *activity.InputSignal, events chan<- Event) {
	options = options.normalized()
	if !options.Enabled {
		return
	}
	machine := newMachine(options.MissThreshold)
	ticker := time.NewTicker(options.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case at := <-ticker.C:
			observation := ObservationOutsideSchedule
			if withinSchedule(at, options) {
				lastInput := time.Time{}
				if input != nil {
					lastInput = input.LastInputAt()
				}
				if !lastInput.IsZero() && at.Sub(lastInput) <= options.IdleGrace {
					observation = ObservationPresent
				} else {
					var err error
					observation, err = localCameraObservation()
					if err != nil {
						log.Printf("Presence check unavailable: %s", err)
					}
				}
			}
			if event := machine.observe(at, observation); event != nil {
				select {
				case events <- *event:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}
