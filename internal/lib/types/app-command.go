package types

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
)

type RequestCommand struct {
	// ShiftHours — на сколько часов назад от текущего часа взять статистику (0 = сейчас).
	ShiftHours   int
	ResponseChan chan<- *AppInfoResponse
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
