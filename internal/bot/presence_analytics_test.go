package bot

import (
	"strings"
	"testing"
	"time"

	"parental-control/internal/lib/types"
)

func TestRenderDailyPresenceReport(t *testing.T) {
	start := time.Date(2026, 9, 7, 10, 0, 0, 0, time.Local)
	response := &types.PresenceResponse{
		Period: types.ActivityDaily, TimeStamp: "2026-09-07",
		PresentSeconds: 6 * 3600, AbsentSeconds: 2 * 3600,
		AbsenceCount: 1, LongestAbsence: 2 * 3600,
		Days: []types.PresenceDaySummary{{
			Date: "2026-09-07", PresentSeconds: 6 * 3600, AbsentSeconds: 2 * 3600,
			AbsenceCount: 1, LongestAbsence: 2 * 3600,
			Absences: []types.PresenceAbsence{{Start: start, End: start.Add(2 * time.Hour), Seconds: 2 * 3600}},
		}},
		HasOlder: true, OlderShift: 1,
	}
	message := renderPresenceReport(response)
	if got := message.Blocks[0].InputRichBlockSectionHeading.Text.PlainText; got != "Presence · Day: 07.09" {
		t.Fatalf("heading = %q", got)
	}
	metrics := message.Blocks[1].InputRichBlockParagraph.Text.PlainText
	for _, want := range []string{"Present: 6h", "Away: 2h", "Presence: 75%", "Absences: 1", "Longest away: 2h"} {
		if !strings.Contains(metrics, want) {
			t.Errorf("metrics %q do not contain %q", metrics, want)
		}
	}
	if table := message.Blocks[2].InputRichBlockTable; table == nil || len(table.Cells) != 2 {
		t.Fatalf("absence table = %#v", table)
	}
	buttons := message.Blocks[3].InputRichBlockButtons.Buttons
	if len(buttons) != 4 || buttons[2].CallbackData != "\fpresence-report-prev|1:1" {
		t.Fatalf("report buttons = %#v", buttons)
	}
}

func TestRenderWeeklyPresenceReportAndEmptyState(t *testing.T) {
	response := &types.PresenceResponse{
		Period: types.ActivityWeekly, TimeStamp: "2026-09-07 – 2026-09-13",
		PresentSeconds: 60, Days: []types.PresenceDaySummary{{Date: "2026-09-07", PresentSeconds: 60}},
	}
	message := renderPresenceReport(response)
	if got := message.Blocks[0].InputRichBlockSectionHeading.Text.PlainText; got != "Presence · Week: 07.09 - 13.09" {
		t.Fatalf("heading = %q", got)
	}
	if table := message.Blocks[2].InputRichBlockTable; table == nil || len(table.Cells) != 2 {
		t.Fatalf("weekly table = %#v", table)
	}

	empty := renderPresenceReport(&types.PresenceResponse{Period: types.ActivityDaily, TimeStamp: "2026-09-06"})
	if got := empty.Blocks[1].InputRichBlockParagraph.Text.PlainText; got != "No Presence Data" {
		t.Fatalf("empty state = %q", got)
	}
}

func TestPresenceReportTargetCodec(t *testing.T) {
	encoded := presenceReportTarget(types.ActivityWeekly, 3)
	period, shift, ok := parsePresenceReportTarget(encoded)
	if !ok || period != types.ActivityWeekly || shift != 3 {
		t.Fatalf("decoded target = %v, %d, %v", period, shift, ok)
	}
	if _, _, ok := parsePresenceReportTarget("0:3"); ok {
		t.Fatal("hourly presence target unexpectedly accepted")
	}
}
