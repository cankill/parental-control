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
	Interval               time.Duration
	Threshold              float64
	ClassifierThreshold    float64
	ClassifierPath         string
	Cooldown               time.Duration
	ActiveLearningMin      float64
	ActiveLearningCooldown time.Duration
	RequiredFrames         int
	OnAlert                func()
}

func NewOptions(interval time.Duration, threshold float64, cooldown time.Duration) Options {
	options := Options{
		Interval: interval, Threshold: threshold, ClassifierThreshold: 0.8,
		Cooldown: cooldown, ActiveLearningMin: 0.4,
		ActiveLearningCooldown: 30 * time.Minute, RequiredFrames: 2,
	}
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

func (o Options) WithClassifier(path string, threshold float64) Options {
	o.ClassifierPath = path
	if threshold >= 0.5 && threshold <= 0.99 {
		o.ClassifierThreshold = threshold
	}
	return o
}

func (o Options) WithActiveLearning(minimum float64, cooldown time.Duration) Options {
	if minimum >= 0.1 && minimum < o.ClassifierThreshold {
		o.ActiveLearningMin = minimum
	}
	if cooldown >= o.Interval {
		o.ActiveLearningCooldown = cooldown
	}
	return o
}

func (o Options) WithAlert(callback func()) Options {
	o.OnAlert = callback
	return o
}

type Candidate struct {
	Record Record
	Photo  []byte
}

type analyzedFrame struct {
	path      string
	detection vision.FaceTouchDetection
}

type monitor struct {
	presence         Presence
	store            *Store
	options          Options
	events           chan<- Candidate
	lastSent         time.Time
	lastLearningSent time.Time
	latched          bool
	clear            int
	collectionPaused bool
	classifierReady  bool
	capture          func() ([]string, error)
	analyze          func(string) (vision.FaceTouchDetection, error)
	now              func() time.Time
}

func Monitor(ctx context.Context, presence Presence, store *Store, options Options, events chan<- Candidate) {
	options = NewOptions(options.Interval, options.Threshold, options.Cooldown).
		WithClassifier(options.ClassifierPath, options.ClassifierThreshold).
		WithActiveLearning(options.ActiveLearningMin, options.ActiveLearningCooldown).
		WithAlert(options.OnAlert)
	classifierReady := modelExists(options.ClassifierPath)
	analyze := vision.AnalyzeFaceTouch
	if classifierReady {
		analyze = func(path string) (vision.FaceTouchDetection, error) {
			return vision.AnalyzeFaceTouchWithModel(path, options.ClassifierPath)
		}
		log.Printf("Face-touch Core ML classifier enabled: %s", options.ClassifierPath)
	}
	m := &monitor{
		presence: presence, store: store, options: options, classifierReady: classifierReady,
		events: events, capture: media.CaptureAnalysisFrames, analyze: analyze, now: time.Now,
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
	if m.collectionPaused {
		return nil
	}
	if m.store != nil {
		summary, err := m.store.Summary()
		if err != nil {
			return err
		}
		if summary.TrainingTargetReached() && !m.classifierReady {
			m.collectionPaused = true
			log.Printf("Face-touch candidate collection paused: training target reached (%d watch, %d ignore)", summary.DatasetWatch, summary.DatasetIgnore)
			return nil
		}
	}
	now := m.now()
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
		frames = append(frames, analyzedFrame{path: path, detection: detection})
	}
	best, reason, ok := selectCandidate(frames, m.options.Threshold, m.options.ClassifierThreshold,
		m.options.ActiveLearningMin, m.classifierReady, m.options.RequiredFrames)
	if !m.observeCandidate(ok) {
		return nil
	}
	m.signalVisualAlert(reason)
	lastSent, cooldown := m.lastSent, m.options.Cooldown
	if reason == ReviewActiveLearning {
		lastSent, cooldown = m.lastLearningSent, m.options.ActiveLearningCooldown
	}
	if !lastSent.IsZero() && now.Sub(lastSent) < cooldown {
		m.latched = true
		return nil
	}
	photo, err := os.ReadFile(best.path)
	if err != nil {
		return fmt.Errorf("read face-touch candidate: %w", err)
	}
	record, err := m.store.CreateCandidateForReview(now, best.detection.Score, diagnosticsFromDetection(best.detection), reason, photo)
	if err != nil {
		return err
	}
	candidate := Candidate{Record: record, Photo: photo}
	select {
	case m.events <- candidate:
		if reason == ReviewActiveLearning {
			m.lastLearningSent = now
		} else {
			m.lastSent = now
		}
		m.latched = true
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *monitor) signalVisualAlert(reason ReviewReason) {
	// The local cue is intentionally stricter than the data-collection path:
	// it only reacts to a positive classifier result. Geometry-only fallback
	// and active-learning candidates must stay silent.
	if m.classifierReady && reason == ReviewAlert && m.options.OnAlert != nil {
		m.options.OnAlert()
	}
}

// observeCandidate reports only the beginning of a gesture. The detector is
// rearmed after two clear checks so one held pinch produces one notification.
func (m *monitor) observeCandidate(hit bool) bool {
	if hit {
		m.clear = 0
		return !m.latched
	}
	m.clear++
	if m.clear >= 2 {
		m.latched = false
	}
	return false
}

func selectCandidate(frames []analyzedFrame, threshold, classifierThreshold, activeLearningMin float64, requireClassifier bool, required int) (analyzedFrame, ReviewReason, bool) {
	if required < 1 {
		required = 1
	}
	best, hits := bestCandidate(frames, threshold, classifierThreshold, 1.01, requireClassifier)
	if hits >= required && best.path != "" {
		return best, ReviewAlert, true
	}
	if requireClassifier {
		best, hits = bestCandidate(frames, threshold, activeLearningMin, classifierThreshold, true)
		if hits >= required && best.path != "" {
			return best, ReviewActiveLearning, true
		}
	}
	return analyzedFrame{}, "", false
}

func bestCandidate(frames []analyzedFrame, geometryThreshold, classifierMin, classifierMax float64, useClassifier bool) (analyzedFrame, int) {
	hits := 0
	best := analyzedFrame{}
	for _, frame := range frames {
		if frame.detection.Score < geometryThreshold {
			continue
		}
		score := frame.detection.Score
		if useClassifier {
			if !frame.detection.ClassifierAvailable || frame.detection.ClassifierScore < classifierMin || frame.detection.ClassifierScore >= classifierMax {
				continue
			}
			score = frame.detection.ClassifierScore
		}
		hits++
		bestScore := best.detection.Score
		if useClassifier {
			bestScore = best.detection.ClassifierScore
		}
		if score > bestScore {
			best = frame
		}
	}
	return best, hits
}

func modelExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func diagnosticsFromDetection(detection vision.FaceTouchDetection) Diagnostics {
	mode := "none"
	classifier := ""
	switch detection.PoseMode {
	case 1:
		mode = "thumb-index"
	case 2:
		mode = "occluded-thumb-index-middle"
	}
	if detection.ClassifierAvailable {
		classifier = ClassifierChinPinchV1
	}
	return Diagnostics{
		PoseMode: mode, ChinProximity: detection.ChinProximity,
		PinchCloseness: detection.PinchCloseness, LandmarkConfidence: detection.LandmarkConfidence,
		NormalizedTipDistance: detection.NormalizedTipDistance, HandScale: detection.HandScale,
		ThumbConfidence: detection.ThumbConfidence, IndexConfidence: detection.IndexConfidence,
		MiddleConfidence: detection.MiddleConfidence,
		RegionX:          detection.RegionX, RegionY: detection.RegionY,
		RegionWidth: detection.RegionWidth, RegionHeight: detection.RegionHeight,
		ClassifierProbability: detection.ClassifierScore,
		Classifier:            classifier,
	}
}
