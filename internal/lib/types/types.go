package types

import (
	"fmt"
	"time"
)

// type ActionType string

// const (
// 	APPLICATION_STARTED  ActionType = "Application Started"
// 	APPLICATION_FINISHED            = "Application Finished"
// 	GAIN_FOCUS                      = "Application Gained Focus"
// 	LOOSE_FOCUS                     = "Application Looses Focus"
// )

type AppInfo struct {
	Identity string
	Duration time.Duration
}

func (ac AppInfo) Dump() string {
	return fmt.Sprintf("%s:\t%s\n", ac.Identity, ac.Duration)
}

func (ac AppInfo) Table() []string {
	return []string{ac.Identity, ac.Duration.String()}
}

type Request struct {
	ResponseChan chan<- []AppInfo
}

type AppCommandType int

type AppCommand interface {
	Type() AppCommandType
}

const (
	Command AppCommandType = iota
	Event
)

type StopCommand struct {
	StoppedChan chan<- bool
}

func (sc StopCommand) Type() AppCommandType {
	return Command
}

type NewAppEvent struct {
	AppName string
}

func (sc NewAppEvent) Type() AppCommandType {
	return Event
}

func Last(ss []string) string {
	return ss[len(ss)-1]
}

func Min(a, b time.Time) time.Time {
	if a.Compare(b) > 0 {
		return b
	}

	return a
}
