package types

type AppCommandType int

type AppCommand interface {
	Type() AppCommandType
}

const (
	Command AppCommandType = iota
	Event
	PeriodCommand
)

type RequestCommand struct {
	// ShiftHours — на сколько часов назад от текущего часа взять статистику (0 = сейчас).
	ShiftHours   int
	ResponseChan chan<- *AppInfoResponse
}

func (sc RequestCommand) Type() AppCommandType {
	return Command
}

// PeriodRequest запрашивает агрегированную статистику за диапазон часов
// [FromShift, ToShift] назад (включительно). Например {0, 23} — последние сутки.
type PeriodRequest struct {
	FromShift    int
	ToShift      int
	ResponseChan chan<- *AppInfoResponse
}

func (pc PeriodRequest) Type() AppCommandType {
	return PeriodCommand
}

type NewAppEvent struct {
	AppName string
}

func (sc NewAppEvent) Type() AppCommandType {
	return Event
}
