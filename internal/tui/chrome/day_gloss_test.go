package chrome

import (
	"testing"
	"time"
)

func TestDayGlossNamesWeekdayAndDistance(t *testing.T) {
	now := time.Date(2026, 9, 26, 15, 0, 0, 0, time.UTC) // a Saturday
	day := func(offset int) time.Time { return time.Date(2026, 9, 26+offset, 0, 0, 0, 0, time.UTC) }
	for offset, want := range map[int]string{
		0:  "Sat, today",
		1:  "Sun, tomorrow",
		-1: "Fri, yesterday",
		6:  "Fri, in 6 days",
		-3: "Wed, 3 days ago",
	} {
		if got := DayGloss(day(offset), now); got != want {
			t.Errorf("DayGloss(%+d) = %q, want %q", offset, got, want)
		}
	}
}
