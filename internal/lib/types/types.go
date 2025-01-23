package types

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	"github.com/olekukonko/tablewriter"
)

type WgKey struct{}
type HostsKey struct{}

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
	table := tablewriter.NewWriter(&buf)
	table.SetHeader([]string{"Application", "Time spent"})
	table.SetBorder(false)
	total := time.Duration(0)

	for _, appInfo := range acs {
		table.Append(appInfo.Table())
		total += appInfo.Duration
	}

	table.SetFooter([]string{"Total", total.String()})
	table.Render()
	return buf.String()
}
