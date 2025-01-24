package types

import "time"

type AppCommandType int

type AppCommand interface {
	Type() AppCommandType
}

const (
	Command AppCommandType = iota
	Event
)

type RequestCommand struct {
	Shift        time.Duration
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
