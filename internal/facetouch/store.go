// Package facetouch detects candidate hand-near-chin events and stores only
// their metadata and user-provided labels. Camera images are transient.
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
)

type Record struct {
	ID         string    `json:"id"`
	CapturedAt time.Time `json:"captured_at"`
	Score      float64   `json:"score"`
	Label      Label     `json:"label"`
	LabeledAt  time.Time `json:"labeled_at,omitempty"`
}

type Summary struct {
	Total    int
	Pending  int
	Watch    int
	Ignore   int
	LatestAt time.Time
}

func (s Summary) Labeled() int { return s.Watch + s.Ignore }

func (s Summary) AcceptedPercent() int {
	if s.Labeled() == 0 {
		return 0
	}
	return (s.Watch*100 + s.Labeled()/2) / s.Labeled()
}

type Store struct {
	mu   sync.Mutex
	root string
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
	return &Store{root: root}, nil
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
		Score: score, Label: LabelPending,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.writeLocked(record); err != nil {
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
		summary.Total++
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
	temporary, err := os.CreateTemp(s.root, ".face-touch-*")
	if err != nil {
		return fmt.Errorf("create temporary face-touch record: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("protect face-touch record: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write face-touch record: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync face-touch record: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close face-touch record: %w", err)
	}
	if err := os.Rename(temporaryPath, s.recordPath(record.ID)); err != nil {
		return fmt.Errorf("replace face-touch record: %w", err)
	}
	return nil
}
