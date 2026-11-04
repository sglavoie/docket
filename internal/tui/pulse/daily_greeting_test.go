package pulse

import (
	"testing"
	"time"
)

func TestDailyGreeting(t *testing.T) {
	at := func(hour int) time.Time { return time.Date(2026, 11, 4, hour, 0, 0, 0, time.Local) }
	cases := []struct {
		pulse WorkspacePulse
		now   time.Time
		want  string
	}{
		{WorkspacePulse{Today: 3, Overdue: 1, Inbox: 2, Done: 9}, at(8), "Good morning · 3 planned for Today · 1 overdue · 2 in Inbox"},
		{WorkspacePulse{Inbox: 1}, at(14), "Good afternoon · 1 in Inbox"},
		{WorkspacePulse{}, at(20), "Good evening · nothing is waiting on you"},
		{WorkspacePulse{Today: 1}, at(2), "Working late · 1 planned for Today"},
	}
	for _, c := range cases {
		if got := DailyGreeting(c.pulse, c.now); got != c.want {
			t.Errorf("DailyGreeting(%+v, %d) = %q, want %q", c.pulse, c.now.Hour(), got, c.want)
		}
	}
}
