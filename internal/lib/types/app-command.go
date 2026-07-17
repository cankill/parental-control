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
	DomainEvent
	DomainCommand
	AppInfoCommand
	ActivityEvent
	ActivityCommand
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

type ActivityRequest struct {
	ShiftHours   int
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
	TimeStamp  string
	ShiftHours int
	Buckets    [12]ActivityBucket
	OlderShift int
	NewerShift int
	HasOlder   bool
	HasNewer   bool
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

type NewAppEvent struct {
	AppName string
}

func (sc NewAppEvent) Type() AppCommandType {
	return Event
}

// DomainTick сообщает, что за истекший интервал был активен домен Domain в течение
// Millis миллисекунд (или пустой домен, если браузер не активен — тик игнорируется).
type DomainTick struct {
	Domain string
	Millis int64
}

func (dt DomainTick) Type() AppCommandType {
	return DomainEvent
}

// DomainRequest запрашивает статистику доменов за час ShiftHours назад.
type DomainRequest struct {
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
