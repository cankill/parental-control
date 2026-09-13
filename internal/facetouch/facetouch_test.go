package facetouch

import (
	"testing"
	"time"
)

func TestSelectCandidateRequiresTwoFramesAtThreshold(t *testing.T) {
	frames := []analyzedFrame{{path: "one.jpg", score: 0.82}, {path: "two.jpg", score: 0.79}, {path: "three.jpg", score: 0.91}}
	path, score, ok := selectCandidate(frames, 0.8, 2)
	if !ok || path != "three.jpg" || score != 0.91 {
		t.Fatalf("candidate = %q %.2f %v", path, score, ok)
	}
	if _, _, ok := selectCandidate(frames[:2], 0.8, 2); ok {
		t.Fatal("single qualifying frame was accepted")
	}
}

func TestMonitorNotifiesOncePerHeldGesture(t *testing.T) {
	m := &monitor{}
	if !m.observeCandidate(true) {
		t.Fatal("first pinch was not accepted")
	}
	m.latched = true
	if m.observeCandidate(true) {
		t.Fatal("held pinch was accepted again")
	}
	if m.observeCandidate(false) {
		t.Fatal("clear frame was accepted")
	}
	if m.observeCandidate(true) {
		t.Fatal("one clear check rearmed the detector")
	}
	if m.observeCandidate(false) || m.observeCandidate(false) {
		t.Fatal("clear checks were accepted")
	}
	if !m.observeCandidate(true) {
		t.Fatal("new pinch was not accepted after two clear checks")
	}
}

func TestStorePersistsLabelsAndSummary(t *testing.T) {
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 9, 13, 9, 0, 0, 1, time.Local)
	first, err := store.Create(start, 0.84)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Create(start.Add(time.Nanosecond), 0.93)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetLabel(first.ID, LabelWatch, start.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetLabel(second.ID, LabelIgnore, start.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	summary, err := store.Summary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.Total != 2 || summary.Watch != 1 || summary.Ignore != 1 || summary.Pending != 0 || summary.AcceptedPercent() != 50 {
		t.Fatalf("summary = %+v", summary)
	}
}

func TestStoreRejectsUnsafeIDsAndLabels(t *testing.T) {
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetLabel("../escape", LabelWatch, time.Now()); err == nil {
		t.Fatal("unsafe id was accepted")
	}
	record, err := store.Create(time.Now(), 0.9)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetLabel(record.ID, Label("other"), time.Now()); err == nil {
		t.Fatal("unknown label was accepted")
	}
}

func TestSummarySeparatesLegacyGenericCandidates(t *testing.T) {
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	legacy := Record{ID: "legacy", CapturedAt: time.Now(), Score: 0.9, Label: LabelWatch}
	store.mu.Lock()
	err = store.writeLocked(legacy)
	store.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(time.Now().Add(time.Nanosecond), 0.91); err != nil {
		t.Fatal(err)
	}
	summary, err := store.Summary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.Total != 1 || summary.Pending != 1 || summary.Legacy != 1 {
		t.Fatalf("summary = %+v", summary)
	}
}

func TestOptionsUseConservativeDefaults(t *testing.T) {
	options := NewOptions(0, 0, 0)
	if options.Interval != 10*time.Second || options.Threshold != 0.8 || options.Cooldown != 30*time.Second || options.RequiredFrames != 2 {
		t.Fatalf("options = %+v", options)
	}
}
