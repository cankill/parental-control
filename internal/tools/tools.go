package tools

import "fmt"

type ActionType string

const (
	APPLICATION_STARTED  ActionType = "Application Started"
	APPLICATION_FINISHED            = "Application Finished"
	GAIN_FOCUS                      = "Application Gained Focus"
	LOOSE_FOCUS                     = "Application Looses Focus"
)

type AppAction struct {
	Identity string
	Action   ActionType
}

func (ac AppAction) Dump() string {
	return fmt.Sprintf("AppAction {identity: %s, action: %s}", ac.Identity, ac.Action)
}
