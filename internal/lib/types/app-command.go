package types

type AppCommandType int

type AppCommand interface {
	Type() AppCommandType
}

const (
	Command AppCommandType = iota
	Event
)

type RequestCommand struct {
	// ShiftHours — на сколько часов назад от текущего часа взять статистику (0 = сейчас).
	ShiftHours   int
	ResponseChan chan<- *AppInfoResponse
}

func (sc RequestCommand) Type() AppCommandType {
	return Command
}

type NewAppEvent struct {
	AppName string
}

func (sc NewAppEvent) Type() AppCommandType {
	return Event
}
