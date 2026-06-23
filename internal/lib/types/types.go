package types

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
)

type WgKey struct{}
type EnvKey struct{}

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

type AppInfos []AppInfo

type AppInfoResponse struct {
	AppInfos  AppInfos
	TimeStamp string
}

func (acs AppInfos) SortByDuration() {
	sort.Slice(acs, func(i, j int) bool {
		return acs[i].Duration < acs[j].Duration
	})
}

func (acs AppInfos) SortByDurationDesc() {
	sort.Slice(acs, func(i, j int) bool {
		return acs[i].Duration > acs[j].Duration
	})
}

func (acs AppInfos) FormatTable() string {
	var buf bytes.Buffer
	// tablewriter v1: рамки отключаем через WithBorders(tw.Off); Header/Footer/
	// Append/Render заменили SetHeader/SetFooter/Append/Render из v0.
	table := tablewriter.NewTable(&buf, tablewriter.WithBorders(tw.Border{
		Left: tw.Off, Right: tw.Off, Top: tw.Off, Bottom: tw.Off,
	}))
	table.Header("Application", "Time spent")
	total := time.Duration(0)

	for _, appInfo := range acs {
		_ = table.Append(appInfo.Table())
		total += appInfo.Duration
	}

	table.Footer("Total", total.String())
	_ = table.Render()
	return buf.String()
}
