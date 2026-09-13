package facetouch

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"parental-control/internal/media"
	"parental-control/internal/vision"
)

type Presence interface {
	Present() bool
}

type Options struct {
	Interval       time.Duration
	Threshold      float64
	Cooldown       time.Duration
	RequiredFrames int
}

func NewOptions(interval time.Duration, threshold float64, cooldown time.Duration) Options {
	options := Options{Interval: interval, Threshold: threshold, Cooldown: cooldown, RequiredFrames: 2}
	if options.Interval < 5*time.Second {
		options.Interval = 10 * time.Second
	}
	if options.Threshold < 0.5 || options.Threshold > 0.99 {
		options.Threshold = 0.8
	}
	if options.Cooldown < options.Interval {
		options.Cooldown = 30 * time.Second
	}
	return options
}

type Candidate struct {
	Record Record
	Photo  []byte
}

type analyzedFrame struct {
	path  string
	score float64
}

type monitor struct {
	presence Presence
	store    *Store
	options  Options
	events   chan<- Candidate
	lastSent time.Time
	capture  func() ([]string, error)
	analyze  func(string) (vision.FaceTouchDetection, error)
	now      func() time.Time
}

func Monitor(ctx context.Context, presence Presence, store *Store, options Options, events chan<- Candidate) {
	m := &monitor{
		presence: presence, store: store, options: NewOptions(options.Interval, options.Threshold, options.Cooldown),
		events: events, capture: media.CaptureAnalysisFrames, analyze: vision.AnalyzeFaceTouch, now: time.Now,
	}
	ticker := time.NewTicker(m.options.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if m.presence == nil || !m.presence.Present() {
				continue
			}
			if err := m.check(ctx); err != nil {
				log.Printf("Face-touch check unavailable: %s", err)
			}
		}
	}
}

func (m *monitor) check(ctx context.Context) error {
	now := m.now()
	if !m.lastSent.IsZero() && now.Sub(m.lastSent) < m.options.Cooldown {
		return nil
	}
	paths, err := m.capture()
	if err != nil {
		return err
	}
	defer func() {
		for _, path := range paths {
			_ = os.Remove(path)
		}
	}()

	frames := make([]analyzedFrame, 0, len(paths))
	for _, path := range paths {
		detection, err := m.analyze(path)
		if err != nil {
			return err
		}
		frames = append(frames, analyzedFrame{path: path, score: detection.Score})
	}
	path, score, ok := selectCandidate(frames, m.options.Threshold, m.options.RequiredFrames)
	if !ok {
		return nil
	}
	photo, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read face-touch candidate: %w", err)
	}
	record, err := m.store.Create(now, score)
	if err != nil {
		return err
	}
	candidate := Candidate{Record: record, Photo: photo}
	select {
	case m.events <- candidate:
		m.lastSent = now
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func selectCandidate(frames []analyzedFrame, threshold float64, required int) (string, float64, bool) {
	if required < 1 {
		required = 1
	}
	hits := 0
	best := analyzedFrame{}
	for _, frame := range frames {
		if frame.score < threshold {
			continue
		}
		hits++
		if frame.score > best.score {
			best = frame
		}
	}
	return best.path, best.score, hits >= required && best.path != ""
}
