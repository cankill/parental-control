// Package facetouch detects candidate pinch-near-chin events and stores a
// private, labeled local dataset for later classifier training.
package facetouch

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"
)

type Label string

const (
	LabelPending Label = "pending"
	LabelWatch   Label = "watch"
	LabelIgnore  Label = "ignore"

	DetectorPinchNearChinV1 = "pinch-near-chin-v1"
	DetectorPinchNearChinV2 = "pinch-near-chin-v2"
)

type Diagnostics struct {
	PoseMode              string  `json:"pose_mode"`
	ChinProximity         float64 `json:"chin_proximity"`
	PinchCloseness        float64 `json:"pinch_closeness"`
	LandmarkConfidence    float64 `json:"landmark_confidence"`
	NormalizedTipDistance float64 `json:"normalized_tip_distance"`
	HandScale             float64 `json:"hand_scale"`
	ThumbConfidence       float64 `json:"thumb_confidence"`
	IndexConfidence       float64 `json:"index_confidence"`
	MiddleConfidence      float64 `json:"middle_confidence"`
}

type Record struct {
	ID          string      `json:"id"`
	CapturedAt  time.Time   `json:"captured_at"`
	Score       float64     `json:"score"`
	Detector    string      `json:"detector,omitempty"`
	ImageFile   string      `json:"image_file,omitempty"`
	Diagnostics Diagnostics `json:"diagnostics,omitempty"`
	Label       Label       `json:"label"`
	LabeledAt   time.Time   `json:"labeled_at,omitempty"`
}

type Summary struct {
	Total          int
	Pending        int
	Watch          int
	Ignore         int
	LatestAt       time.Time
	Legacy         int
	Dataset        int
	DatasetLabeled int
}

func (s Summary) Labeled() int { return s.Watch + s.Ignore }

func (s Summary) AcceptedPercent() int {
	if s.Labeled() == 0 {
		return 0
	}
	return (s.Watch*100 + s.Labeled()/2) / s.Labeled()
}

type Store struct {
	mu        sync.Mutex
	root      string
	imageRoot string
}

var validID = regexp.MustCompile(`^[a-z0-9]+$`)

func StorePath() string {
	database := os.Getenv("PARENTAL_CONTROL_DB")
	if database == "" {
		database = "./database"
	}
	return filepath.Join(database, "face-touch")
}

func OpenStore(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, fmt.Errorf("create face-touch storage: %w", err)
	}
	if err := os.Chmod(root, 0700); err != nil {
		return nil, fmt.Errorf("protect face-touch storage: %w", err)
	}
	imageRoot := filepath.Join(root, "images")
	if err := os.MkdirAll(imageRoot, 0700); err != nil {
		return nil, fmt.Errorf("create face-touch image storage: %w", err)
	}
	if err := os.Chmod(imageRoot, 0700); err != nil {
		return nil, fmt.Errorf("protect face-touch image storage: %w", err)
	}
	return &Store{root: root, imageRoot: imageRoot}, nil
}

func (s *Store) Create(at time.Time, score float64) (Record, error) {
	if s == nil {
		return Record{}, errors.New("face-touch storage is unavailable")
	}
	if at.IsZero() {
		at = time.Now()
	}
	record := Record{
		ID: strconv.FormatInt(at.UnixNano(), 36), CapturedAt: at,
		Score: score, Detector: DetectorPinchNearChinV2, Label: LabelPending,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.writeLocked(record); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s *Store) CreateCandidate(at time.Time, score float64, diagnostics Diagnostics, photo []byte) (Record, error) {
	if s == nil {
		return Record{}, errors.New("face-touch storage is unavailable")
	}
	if len(photo) == 0 {
		return Record{}, errors.New("face-touch candidate photo is empty")
	}
	if at.IsZero() {
		at = time.Now()
	}
	id := strconv.FormatInt(at.UnixNano(), 36)
	record := Record{
		ID: id, CapturedAt: at, Score: score, Detector: DetectorPinchNearChinV2,
		ImageFile: filepath.ToSlash(filepath.Join("images", id+".jpg")), Diagnostics: diagnostics,
		Label: LabelPending,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	imagePath := s.imagePath(id)
	if err := writeAtomic(s.imageRoot, ".face-touch-image-*", imagePath, photo); err != nil {
		return Record{}, fmt.Errorf("store face-touch candidate photo: %w", err)
	}
	if err := s.writeLocked(record); err != nil {
		_ = os.Remove(imagePath)
		_ = os.Remove(s.datasetMetadataPath(id))
		return Record{}, err
	}
	return record, nil
}

func (s *Store) SetLabel(id string, label Label, at time.Time) (Record, error) {
	if s == nil {
		return Record{}, errors.New("face-touch storage is unavailable")
	}
	if !validID.MatchString(id) {
		return Record{}, errors.New("invalid face-touch event id")
	}
	if label != LabelWatch && label != LabelIgnore {
		return Record{}, errors.New("invalid face-touch label")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.readLocked(id)
	if err != nil {
		return Record{}, err
	}
	record.Label = label
	record.LabeledAt = at
	if record.LabeledAt.IsZero() {
		record.LabeledAt = time.Now()
	}
	if err := s.writeLocked(record); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s *Store) ReadPhoto(id string) ([]byte, error) {
	if s == nil {
		return nil, errors.New("face-touch storage is unavailable")
	}
	if !validID.MatchString(id) {
		return nil, errors.New("invalid face-touch event id")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, err := s.readLocked(id)
	if err != nil {
		return nil, err
	}
	if record.ImageFile == "" {
		return nil, errors.New("face-touch event has no stored photo")
	}
	photo, err := os.ReadFile(s.imagePath(id))
	if err != nil {
		return nil, fmt.Errorf("read face-touch candidate photo: %w", err)
	}
	return photo, nil
}

func (s *Store) Summary() (Summary, error) {
	if s == nil {
		return Summary{}, errors.New("face-touch storage is unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return Summary{}, fmt.Errorf("list face-touch records: %w", err)
	}
	var summary Summary
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := entry.Name()[:len(entry.Name())-len(".json")]
		record, err := s.readLocked(id)
		if err != nil {
			continue
		}
		if record.Detector != DetectorPinchNearChinV1 && record.Detector != DetectorPinchNearChinV2 {
			summary.Legacy++
			continue
		}
		summary.Total++
		if record.ImageFile != "" {
			summary.Dataset++
			if record.Label == LabelWatch || record.Label == LabelIgnore {
				summary.DatasetLabeled++
			}
		}
		switch record.Label {
		case LabelWatch:
			summary.Watch++
		case LabelIgnore:
			summary.Ignore++
		default:
			summary.Pending++
		}
		if record.CapturedAt.After(summary.LatestAt) {
			summary.LatestAt = record.CapturedAt
		}
	}
	return summary, nil
}

func (s *Store) recordPath(id string) string { return filepath.Join(s.root, id+".json") }

func (s *Store) imagePath(id string) string { return filepath.Join(s.imageRoot, id+".jpg") }

func (s *Store) datasetMetadataPath(id string) string { return filepath.Join(s.imageRoot, id+".json") }

func (s *Store) readLocked(id string) (Record, error) {
	data, err := os.ReadFile(s.recordPath(id))
	if err != nil {
		return Record{}, fmt.Errorf("read face-touch record: %w", err)
	}
	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return Record{}, fmt.Errorf("decode face-touch record: %w", err)
	}
	return record, nil
}

func (s *Store) writeLocked(record Record) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encode face-touch record: %w", err)
	}
	if record.ImageFile != "" {
		if err := writeAtomic(s.imageRoot, ".face-touch-metadata-*", s.datasetMetadataPath(record.ID), data); err != nil {
			return fmt.Errorf("write face-touch dataset metadata: %w", err)
		}
	}
	if err := writeAtomic(s.root, ".face-touch-*", s.recordPath(record.ID), data); err != nil {
		return fmt.Errorf("write face-touch record: %w", err)
	}
	return nil
}

func writeAtomic(directory, pattern, destination string, data []byte) error {
	temporary, err := os.CreateTemp(directory, pattern)
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return err
	}
	return nil
}
