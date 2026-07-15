package types

import (
	"testing"
	"time"
)

func TestLast(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"a", "b", "c"}, "c"},
		{[]string{"only"}, "only"},
		{[]string{}, ""},  // не должно паниковать
		{nil, ""},         // не должно паниковать
		{[]string{""}, ""}, // strings.Split("") даёт [""]
	}
	for _, c := range cases {
		if got := Last(c.in); got != c.want {
			t.Errorf("Last(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMin(t *testing.T) {
	early := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	late := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	if !Min(early, late).Equal(early) {
		t.Error("Min(early, late) should be early")
	}
	if !Min(late, early).Equal(early) {
		t.Error("Min(late, early) should be early")
	}
	if !Min(early, early).Equal(early) {
		t.Error("Min(early, early) should be early")
	}
}
