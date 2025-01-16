package types

import "time"

func Last(ss []string) string {
	return ss[len(ss)-1]
}

func Min(a, b time.Time) time.Time {
	if a.Compare(b) > 0 {
		return b
	}

	return a
}
