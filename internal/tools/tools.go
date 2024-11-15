package tools

import "fmt"

// type ActionType string

// const (
// 	APPLICATION_STARTED  ActionType = "Application Started"
// 	APPLICATION_FINISHED            = "Application Finished"
// 	GAIN_FOCUS                      = "Application Gained Focus"
// 	LOOSE_FOCUS                     = "Application Looses Focus"
// )

type AppInfo struct {
	Identity string
	Time     int64
}

func (ac AppInfo) Dump() string {
	return fmt.Sprintf("%s runs for: %d [ms]\n", ac.Identity, ac.Time)
}

type Request struct {
	ResponseChan chan<- []AppInfo
}
