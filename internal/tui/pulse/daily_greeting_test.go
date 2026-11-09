package pulse

import (
	"testing"
	"time"
)

func TestDailyGreeting(t *testing.T) {
	at := func(hour int) time.Time { return time.Date(2026, 11, 4, hour, 0, 0, 0, time.Local) }
	friday := func(hour int) time.Time { return time.Date(2026, 11, 6, hour, 0, 0, 0, time.Local) }
	cases := []struct {
		pulse WorkspacePulse
		now   time.Time
		want  string
	}{
		{WorkspacePulse{Today: 3, Overdue: 1, Inbox: 2, Done: 9}, at(8), "Good morning · overdue work first (gm Start the Day)"},
		{WorkspacePulse{Inbox: 1}, at(14), "Good afternoon · the Inbox wants sorting (I triage)"},
		{WorkspacePulse{}, at(20), "Good evening · nothing is waiting on you"},
		{WorkspacePulse{Today: 1}, at(2), "Working late · the plan is ready (gt Today)"},
		{WorkspacePulse{Today: 2, Inbox: 3}, at(9), "Good morning · the Inbox wants sorting (I triage)"},
		{WorkspacePulse{Today: 2}, at(9), "Good morning · the plan is ready (gt Today)"},
		{WorkspacePulse{}, at(8), "Good morning · nothing planned yet (gm Start the Day)"},
		{WorkspacePulse{}, at(13), "Good afternoon · nothing planned yet (gt, then I to plan the day)"},
		{WorkspacePulse{Overdue: 2}, at(14), "Good afternoon · overdue work first (7 Due)"},
		{WorkspacePulse{Done: 2}, at(8), "Good morning · nothing is waiting on you"},
		{WorkspacePulse{Today: 2, Done: 3}, at(19), "Good evening · the plan is ready to close (ge Close the Day)"},
		{WorkspacePulse{Done: 4}, friday(16), "Good afternoon · a quiet moment to look back (gW Review the Week)"},
		{WorkspacePulse{Done: 4}, friday(10), "Good morning · nothing is waiting on you"},
	}
	for _, c := range cases {
		if got := DailyGreeting(c.pulse, c.now); got != c.want {
			t.Errorf("DailyGreeting(%+v, %d) = %q, want %q", c.pulse, c.now.Hour(), got, c.want)
		}
	}
}
