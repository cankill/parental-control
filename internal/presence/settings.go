package presence

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const settingsFileName = "presence-settings.json"

type ScheduleMode string

const (
	ScheduleAlways ScheduleMode = "always"
	ScheduleUntil  ScheduleMode = "until"
	ScheduleDaily  ScheduleMode = "daily"
)

type Policy struct {
	Enabled     bool         `json:"enabled"`
	Mode        ScheduleMode `json:"mode"`
	Until       time.Time    `json:"until,omitempty"`
	StartMinute int          `json:"start_minute,omitempty"`
	EndMinute   int          `json:"end_minute,omitempty"`
	WorkDays    []int        `json:"work_days,omitempty"`
}

type Snapshot struct {
	Policy        Policy
	ActiveNow     bool
	Interval      time.Duration
	IdleGrace     time.Duration
	MissThreshold int
}

type persistedSettings struct {
	Version int    `json:"version"`
	Policy  Policy `json:"policy"`
}

type Controller struct {
	mu      sync.Mutex
	path    string
	options Options
	policy  Policy
}

var (
	clockPattern    = regexp.MustCompile(`^([01]\d|2[0-3]):([0-5]\d)$`)
	rangePattern    = regexp.MustCompile(`^([01]\d|2[0-3]):([0-5]\d)-([01]\d|2[0-3]):([0-5]\d)$`)
	durationPattern = regexp.MustCompile(`(?i)^(?:(\d+)d)?(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?$`)
)

func SettingsPath() string {
	database := os.Getenv("PARENTAL_CONTROL_DB")
	if database == "" {
		database = "./database"
	}
	return filepath.Join(database, settingsFileName)
}

func defaultPolicy(options Options) Policy {
	policy := Policy{
		Enabled:     options.Enabled,
		Mode:        ScheduleDaily,
		StartMinute: options.StartHour * 60,
		EndMinute:   options.EndHour * 60,
		WorkDays:    append([]int(nil), options.WorkDays...),
	}
	return normalizePolicy(policy)
}

// OpenController loads Telegram-managed settings when present and otherwise
// starts from deployment defaults. A non-nil controller is returned even if a
// malformed settings file is encountered, so monitoring can still be managed.
func OpenController(path string, options Options) (*Controller, error) {
	options = options.normalized()
	controller := &Controller{path: path, options: options, policy: defaultPolicy(options)}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return controller, nil
	}
	if err != nil {
		return controller, fmt.Errorf("read presence settings: %w", err)
	}
	var stored persistedSettings
	if err := json.Unmarshal(data, &stored); err != nil {
		return controller, fmt.Errorf("decode presence settings: %w", err)
	}
	if stored.Version != 1 {
		return controller, fmt.Errorf("unsupported presence settings version %d", stored.Version)
	}
	if err := validatePolicy(stored.Policy); err != nil {
		return controller, fmt.Errorf("validate presence settings: %w", err)
	}
	controller.policy = normalizePolicy(stored.Policy)
	return controller, nil
}

func normalizePolicy(policy Policy) Policy {
	if policy.Mode == "" {
		policy.Mode = ScheduleAlways
	}
	seen := make(map[int]struct{}, len(policy.WorkDays))
	days := make([]int, 0, len(policy.WorkDays))
	for _, day := range policy.WorkDays {
		if day < 0 || day > 6 {
			continue
		}
		if _, exists := seen[day]; exists {
			continue
		}
		seen[day] = struct{}{}
		days = append(days, day)
	}
	sort.Ints(days)
	policy.WorkDays = days
	return policy
}

func validatePolicy(policy Policy) error {
	switch policy.Mode {
	case ScheduleAlways:
	case ScheduleUntil:
		if policy.Until.IsZero() {
			return errors.New("until schedule has no end time")
		}
	case ScheduleDaily:
		if policy.StartMinute < 0 || policy.StartMinute >= 24*60 || policy.EndMinute < 0 || policy.EndMinute > 24*60 {
			return errors.New("daily schedule is outside a 24-hour day")
		}
	default:
		return fmt.Errorf("unknown schedule mode %q", policy.Mode)
	}
	for _, day := range policy.WorkDays {
		if day < 0 || day > 6 {
			return fmt.Errorf("invalid weekday %d", day)
		}
	}
	return nil
}

func (c *Controller) Enable(expression string, now time.Time) (Snapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	updated, err := applyExpression(c.policy, expression, now)
	if err != nil {
		return Snapshot{}, err
	}
	if err := validatePolicy(updated); err != nil {
		return Snapshot{}, err
	}
	if err := c.saveLocked(updated); err != nil {
		return Snapshot{}, err
	}
	c.policy = normalizePolicy(updated)
	return c.snapshotLocked(now), nil
}

func (c *Controller) Disable(now time.Time) (Snapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	updated := c.policy
	updated.Enabled = false
	if err := c.saveLocked(updated); err != nil {
		return Snapshot{}, err
	}
	c.policy = updated
	return c.snapshotLocked(now), nil
}

func (c *Controller) Snapshot(now time.Time) Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.policy.Enabled && c.policy.Mode == ScheduleUntil && !now.Before(c.policy.Until) {
		updated := c.policy
		updated.Enabled = false
		c.policy = updated
		if err := c.saveLocked(updated); err != nil {
			log.Printf("Persist expired presence settings: %s", err)
		}
	}
	return c.snapshotLocked(now)
}

func (c *Controller) snapshotLocked(now time.Time) Snapshot {
	policy := c.policy
	policy.WorkDays = append([]int(nil), policy.WorkDays...)
	return Snapshot{
		Policy:        policy,
		ActiveNow:     policyActiveAt(policy, now),
		Interval:      c.options.Interval,
		IdleGrace:     c.options.IdleGrace,
		MissThreshold: c.options.MissThreshold,
	}
}

func (c *Controller) saveLocked(policy Policy) error {
	if c.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0700); err != nil {
		return fmt.Errorf("create presence settings directory: %w", err)
	}
	data, err := json.MarshalIndent(persistedSettings{Version: 1, Policy: policy}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode presence settings: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(c.path), ".presence-settings-*")
	if err != nil {
		return fmt.Errorf("create temporary presence settings: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("protect presence settings: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write presence settings: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync presence settings: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close presence settings: %w", err)
	}
	if err := os.Rename(temporaryName, c.path); err != nil {
		return fmt.Errorf("replace presence settings: %w", err)
	}
	return nil
}

func applyExpression(current Policy, expression string, now time.Time) (Policy, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return Policy{Enabled: true, Mode: ScheduleAlways}, nil
	}
	updated := current
	updated.Enabled = true
	if updated.Mode == ScheduleUntil && !now.Before(updated.Until) {
		updated.Mode = ScheduleAlways
		updated.Until = time.Time{}
	}

	haveTime := false
	haveDays := false
	for _, token := range strings.Fields(expression) {
		if match := rangePattern.FindStringSubmatch(token); match != nil {
			if haveTime {
				return Policy{}, errors.New("specify only one time or duration expression")
			}
			updated.Mode = ScheduleDaily
			updated.Until = time.Time{}
			updated.StartMinute = mustClockMinutes(match[1], match[2])
			updated.EndMinute = mustClockMinutes(match[3], match[4])
			haveTime = true
			continue
		}
		if match := clockPattern.FindStringSubmatch(token); match != nil {
			if haveTime {
				return Policy{}, errors.New("specify only one time or duration expression")
			}
			target := time.Date(now.Year(), now.Month(), now.Day(), mustAtoi(match[1]), mustAtoi(match[2]), 0, 0, now.Location())
			if !target.After(now) {
				target = target.AddDate(0, 0, 1)
			}
			updated.Mode = ScheduleUntil
			updated.Until = target
			haveTime = true
			continue
		}
		if duration, ok, err := parseExtendedDuration(token); err != nil {
			return Policy{}, err
		} else if ok {
			if haveTime {
				return Policy{}, errors.New("specify only one time or duration expression")
			}
			updated.Mode = ScheduleUntil
			updated.Until = now.Add(duration)
			haveTime = true
			continue
		}
		if days, ok := parseWeekdays(token); ok {
			if haveDays {
				return Policy{}, errors.New("specify weekdays only once")
			}
			updated.WorkDays = days
			haveDays = true
			continue
		}
		return Policy{}, fmt.Errorf("invalid presence period %q", token)
	}
	return normalizePolicy(updated), nil
}

func parseExtendedDuration(value string) (time.Duration, bool, error) {
	match := durationPattern.FindStringSubmatch(value)
	if match == nil || value == "" {
		return 0, false, nil
	}
	units := []time.Duration{24 * time.Hour, time.Hour, time.Minute, time.Second}
	var total time.Duration
	for i, raw := range match[1:] {
		if raw == "" {
			continue
		}
		amount, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || amount > int64((time.Duration(math.MaxInt64)-total)/units[i]) {
			return 0, true, fmt.Errorf("presence duration %q is too large", value)
		}
		total += time.Duration(amount) * units[i]
	}
	if total <= 0 {
		return 0, true, errors.New("presence duration must be greater than zero")
	}
	return total, true, nil
}

func parseWeekdays(value string) ([]int, bool) {
	names := map[string]int{"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6}
	parts := strings.Split(strings.ToLower(value), ",")
	days := make([]int, 0, len(parts))
	for _, part := range parts {
		day, ok := names[strings.TrimSpace(part)]
		if !ok {
			return nil, false
		}
		days = append(days, day)
	}
	return normalizePolicy(Policy{WorkDays: days}).WorkDays, true
}

func mustClockMinutes(hour, minute string) int {
	return mustAtoi(hour)*60 + mustAtoi(minute)
}

func mustAtoi(value string) int {
	parsed, _ := strconv.Atoi(value)
	return parsed
}

func policyActiveAt(policy Policy, at time.Time) bool {
	if !policy.Enabled {
		return false
	}
	if policy.Mode == ScheduleUntil && !at.Before(policy.Until) {
		return false
	}
	minute := at.Hour()*60 + at.Minute()
	scheduleDay := int(at.Weekday())
	if policy.Mode == ScheduleDaily && policy.StartMinute > policy.EndMinute && minute < policy.EndMinute {
		scheduleDay = int(at.AddDate(0, 0, -1).Weekday())
	}
	if len(policy.WorkDays) > 0 && !containsDay(policy.WorkDays, scheduleDay) {
		return false
	}
	if policy.Mode != ScheduleDaily || policy.StartMinute == policy.EndMinute {
		return true
	}
	if policy.StartMinute < policy.EndMinute {
		return minute >= policy.StartMinute && minute < policy.EndMinute
	}
	return minute >= policy.StartMinute || minute < policy.EndMinute
}

func containsDay(days []int, value int) bool {
	for _, day := range days {
		if day == value {
			return true
		}
	}
	return false
}
