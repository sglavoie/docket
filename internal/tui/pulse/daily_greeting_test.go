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
		{WorkspacePulse{Today: 3, Overdue: 1, Inbox: 2, Done: 9}, at(8), "Good morning · start with the 1 overdue (Task → Start the Day)"},
		{WorkspacePulse{Inbox: 1}, at(14), "Good afternoon · 1 in Inbox to sort (I triage)"},
		{WorkspacePulse{}, at(20), "Good evening · nothing is waiting on you"},
		{WorkspacePulse{Today: 1}, at(2), "Working late · 1 task planned and ready (gt Today)"},
		{WorkspacePulse{Today: 2, Inbox: 3}, at(9), "Good morning · 3 in Inbox to sort (I triage)"},
		{WorkspacePulse{Today: 2}, at(9), "Good morning · 2 tasks planned and ready (gt Today)"},
		{WorkspacePulse{}, at(8), "Good morning · nothing planned yet (Task → Start the Day)"},
		{WorkspacePulse{}, at(13), "Good afternoon · nothing planned yet (gt, then I to plan the day)"},
		{WorkspacePulse{Overdue: 2}, at(14), "Good afternoon · start with the 2 overdue (7 Due)"},
		{WorkspacePulse{Done: 2}, at(8), "Good morning · nothing is waiting on you"},
	}
	for _, c := range cases {
		if got := DailyGreeting(c.pulse, c.now); got != c.want {
			t.Errorf("DailyGreeting(%+v, %d) = %q, want %q", c.pulse, c.now.Hour(), got, c.want)
		}
	}
}
