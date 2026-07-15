package types

import (
	"strings"
	"testing"
	"time"
)

func TestSortByDurationDesc(t *testing.T) {
	acs := AppInfos{
		{Identity: "small", Duration: 5 * time.Minute},
		{Identity: "big", Duration: 60 * time.Minute},
		{Identity: "mid", Duration: 30 * time.Minute},
	}
	acs.SortByDurationDesc()
	if acs[0].Identity != "big" || acs[1].Identity != "mid" || acs[2].Identity != "small" {
		t.Fatalf("desc order wrong: %v", acs)
	}
}

func TestSortByDuration(t *testing.T) {
	acs := AppInfos{
		{Identity: "big", Duration: 60 * time.Minute},
		{Identity: "small", Duration: 5 * time.Minute},
	}
	acs.SortByDuration()
	if acs[0].Identity != "small" || acs[1].Identity != "big" {
		t.Fatalf("asc order wrong: %v", acs)
	}
}

// FormatTable должен обрезать длинные имена (регрессия: 32-символьный Chrome
// extension id растягивал колонку и ломал таблицу в Telegram).
func TestFormatTableTruncatesLongNames(t *testing.T) {
	longName := "Adnlfjpnmidfimlkaohpidplnoimahfh" // 32 символа
	acs := AppInfos{
		{Identity: longName, Duration: 7 * time.Second},
		{Identity: "Chrome", Duration: 90 * time.Minute},
	}
	out := acs.FormatTable()

	if out == "" {
		t.Fatal("empty table")
	}
	// Полное длинное имя не должно присутствовать целиком (обрезано с …).
	if strings.Contains(out, longName) {
		t.Errorf("long name not truncated:\n%s", out)
	}
	// Ни одна строка не должна быть чрезмерно широкой для мобильного экрана.
	for _, line := range strings.Split(out, "\n") {
		if len([]rune(line)) > 60 {
			t.Errorf("line too wide (%d runes): %q", len([]rune(line)), line)
		}
	}
	// Заголовки и итог на месте (tablewriter рендерит заголовок в верхнем регистре).
	low := strings.ToLower(out)
	if !strings.Contains(low, "application") || !strings.Contains(low, "total") {
		t.Errorf("missing header/footer:\n%s", out)
	}
}

func TestFormatTableEmpty(t *testing.T) {
	empty := AppInfos{}
	if empty.FormatTable() == "" {
		t.Error("empty AppInfos should still render a header/total table, not empty string")
	}
}

// FormatTableMarked помечает ● только строку активного приложения.
func TestFormatTableMarked(t *testing.T) {
	acs := AppInfos{
		{Identity: "Chrome", Duration: 60 * time.Minute},
		{Identity: "Terminal", Duration: 30 * time.Minute},
	}
	out := acs.FormatTableMarked("Terminal")
	if !strings.Contains(out, "● Terminal") {
		t.Errorf("active app not marked:\n%s", out)
	}
	if strings.Contains(out, "● Chrome") {
		t.Errorf("non-active app wrongly marked:\n%s", out)
	}

	// Пустое имя — без пометки.
	if strings.Contains(acs.FormatTableMarked(""), "●") {
		t.Error("no marker expected for empty active name")
	}
}
