package types

import "time"

type AppCommandType int

type AppCommand interface {
	Type() AppCommandType
}

const (
	Command AppCommandType = iota
	Event
	DayCommand
	WeekCommand
	DomainEvent
	DomainCommand
	AppInfoCommand
	ActivityEvent
	ActivityCommand
	PresenceEvent
	PresenceCommand
)

type RequestCommand struct {
	// ShiftHours — на сколько часов назад от текущего часа взять статистику (0 = сейчас).
	ShiftHours   int
	ResponseChan chan<- *AppInfoResponse
}

type ActivityKind uint8

const (
	ActivityNone ActivityKind = iota
	ActivityKeyboard
	ActivityMouse
	ActivityBoth
)

type ActivitySample struct {
	At   time.Time
	Kind ActivityKind
}

type ActivityBatch struct{ Samples []ActivitySample }

func (b ActivityBatch) Type() AppCommandType { return ActivityEvent }

type ActivityPeriod uint8

const (
	ActivityHourly ActivityPeriod = iota
	ActivityDaily
	ActivityWeekly
)

type ActivityRequest struct {
	Period       ActivityPeriod
	Shift        int
	ResponseChan chan<- *ActivityResponse
}

func (r ActivityRequest) Type() AppCommandType { return ActivityCommand }

type ActivityBucket struct {
	KeyboardOnlySeconds int `json:"keyboard_only_seconds"`
	MouseOnlySeconds    int `json:"mouse_only_seconds"`
	BothSeconds         int `json:"both_seconds"`
}

func (b ActivityBucket) ActiveSeconds() int {
	return b.KeyboardOnlySeconds + b.MouseOnlySeconds + b.BothSeconds
}

type ActivityResponse struct {
	Period        ActivityPeriod
	TimeStamp     string
	Shift         int
	BucketSeconds int
	Buckets       []ActivityBucket
	PeakSeconds   int
	OlderShift    int
	NewerShift    int
	HasOlder      bool
	HasNewer      bool
}

type PresenceKind uint8

const (
	PresencePresent PresenceKind = iota + 1
	PresenceMissing
	PresenceUnavailable
)

// PresenceSample is a privacy-preserving monitoring observation. It contains
// only a timestamp, aggregate state, and covered duration; no images or
// biometric data are retained.
type PresenceSample struct {
	At            time.Time
	Kind          PresenceKind
	Seconds       int
	MissThreshold int
}

func (s PresenceSample) Type() AppCommandType { return PresenceEvent }

type PresenceRequest struct {
	Period       ActivityPeriod
	Shift        int
	ResponseChan chan<- *PresenceResponse
}

func (r PresenceRequest) Type() AppCommandType { return PresenceCommand }

type PresenceAbsence struct {
	Start   time.Time
	End     time.Time
	Seconds int
}

type PresenceDaySummary struct {
	Date               string
	PresentSeconds     int
	AbsentSeconds      int
	UnavailableSeconds int
	AbsenceCount       int
	LongestAbsence     int
	Absences           []PresenceAbsence
}

func (s PresenceDaySummary) MonitoredSeconds() int {
	return s.PresentSeconds + s.AbsentSeconds + s.UnavailableSeconds
}

func (s PresenceDaySummary) PresencePercent() int {
	known := s.PresentSeconds + s.AbsentSeconds
	if known == 0 {
		return 0
	}
	return (s.PresentSeconds*100 + known/2) / known
}

type PresenceResponse struct {
	Period             ActivityPeriod
	TimeStamp          string
	Shift              int
	Days               []PresenceDaySummary
	PresentSeconds     int
	AbsentSeconds      int
	UnavailableSeconds int
	AbsenceCount       int
	LongestAbsence     int
	OlderShift         int
	NewerShift         int
	HasOlder           bool
	HasNewer           bool
}

func (r PresenceResponse) MonitoredSeconds() int {
	return r.PresentSeconds + r.AbsentSeconds + r.UnavailableSeconds
}

func (r PresenceResponse) PresencePercent() int {
	known := r.PresentSeconds + r.AbsentSeconds
	if known == 0 {
		return 0
	}
	return (r.PresentSeconds*100 + known/2) / known
}

func (sc RequestCommand) Type() AppCommandType {
	return Command
}

// DayRequest запрашивает агрегированную статистику за календарный день,
// отстоящий на DayShift суток назад (0 = сегодня).
type DayRequest struct {
	DayShift     int
	ResponseChan chan<- *AppInfoResponse
}

func (dc DayRequest) Type() AppCommandType {
	return DayCommand
}

// WeekRequest запрашивает агрегированную статистику за календарную неделю
// (понедельник–воскресенье), отстоящую на WeekShift недель назад.
type WeekRequest struct {
	WeekShift    int
	ResponseChan chan<- *AppInfoResponse
}

func (wr WeekRequest) Type() AppCommandType {
	return WeekCommand
}

type NewAppEvent struct {
	AppName string
}

func (sc NewAppEvent) Type() AppCommandType {
	return Event
}

// DomainTick records one browser observation. Empty/internal tabs and session
// context are retained in raw storage and filtered only while building reports.
type DomainTick struct {
	At              time.Time
	BrowserBundleID string
	Domain          string
	// RawMillis is the complete wall-clock gap since the preceding poll. Millis
	// is the measured portion attributable to this observation; long sleep or
	// scheduling gaps are retained in RawMillis but not credited as activity.
	RawMillis int64
	Millis    int64
}

func (dt DomainTick) Type() AppCommandType {
	return DomainEvent
}

// DomainRequest запрашивает статистику доменов за час ShiftHours назад.
type DomainRequest struct {
	Period       ActivityPeriod
	ShiftHours   int
	ResponseChan chan<- *AppInfoResponse
}

func (dr DomainRequest) Type() AppCommandType {
	return DomainCommand
}

// AppInfoQuery запрашивает метаданные приложения из словаря по отображаемому имени
// Name. Ответ — готовый текст (форматируется в обработчике), чтобы пакет types не
// зависел от пакета appinfo.
type AppInfoQuery struct {
	Name         string
	ResponseChan chan<- string
}

func (q AppInfoQuery) Type() AppCommandType {
	return AppInfoCommand
}
