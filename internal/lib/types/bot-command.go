package types

type BotCommandType int

type BotCommand interface {
	Type() BotCommandType
}

const (
	BCommand BotCommandType = iota
)

type BotStopCommand struct {
	StoppedChan chan<- bool
}

func (sc BotStopCommand) Type() BotCommandType {
	return BCommand
}
