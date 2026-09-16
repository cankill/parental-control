package facetouch

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"parental-control/internal/vision"
)

func TestSelectCandidateRequiresTwoFramesAtThreshold(t *testing.T) {
	frames := []analyzedFrame{
		{path: "one.jpg", detection: vision.FaceTouchDetection{Score: 0.82}},
		{path: "two.jpg", detection: vision.FaceTouchDetection{Score: 0.79}},
		{path: "three.jpg", detection: vision.FaceTouchDetection{Score: 0.91}},
	}
	best, ok := selectCandidate(frames, 0.8, 2)
	if !ok || best.path != "three.jpg" || best.detection.Score != 0.91 {
		t.Fatalf("candidate = %+v %v", best, ok)
	}
	if _, ok := selectCandidate(frames[:2], 0.8, 2); ok {
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

func TestStoreRetainsLabeledPhotoAndDiagnosticMetadata(t *testing.T) {
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 15, 9, 0, 0, 0, time.Local)
	diagnostics := Diagnostics{PoseMode: "thumb-index", ChinProximity: 0.8, PinchCloseness: 0.7}
	record, err := store.CreateCandidate(at, 0.76, diagnostics, []byte("jpeg-data"))
	if err != nil {
		t.Fatal(err)
	}
	image, err := os.ReadFile(store.imagePath(record.ID))
	if err != nil || string(image) != "jpeg-data" {
		t.Fatalf("stored image = %q, %v", image, err)
	}
	info, err := os.Stat(store.imagePath(record.ID))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("stored image mode = %v, %v", info.Mode().Perm(), err)
	}
	if _, err := store.SetLabel(record.ID, LabelWatch, at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	metadata, err := os.ReadFile(store.datasetMetadataPath(record.ID))
	if err != nil {
		t.Fatal(err)
	}
	var labeled Record
	if err := json.Unmarshal(metadata, &labeled); err != nil {
		t.Fatal(err)
	}
	if labeled.Label != LabelWatch || labeled.Diagnostics.PoseMode != "thumb-index" || labeled.ImageFile == "" {
		t.Fatalf("dataset metadata = %+v", labeled)
	}
	photo, err := store.ReadPhoto(record.ID)
	if err != nil || string(photo) != "jpeg-data" {
		t.Fatalf("read stored photo = %q, %v", photo, err)
	}
	if _, err := store.ReadPhoto("../escape"); err == nil {
		t.Fatal("unsafe photo id was accepted")
	}
	summary, err := store.Summary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.Dataset != 1 || summary.DatasetLabeled != 1 {
		t.Fatalf("dataset summary = %+v", summary)
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
