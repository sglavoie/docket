package pulse

import (
	"time"
)

// DailyGreeting is the line the first interactive launch of a local day
// opens with: a salutation for the hour, then the one move the day asks for
// first. The pulse already counts Today, overdue, and Inbox on the title row,
// so the greeting names no number: it chooses among them — overdue work before
// an untriaged Inbox before the plan — and names only that, with the key that
// starts it. The day's passes are the go-to chord's Day run, so the move it names
// is a key rather than a menu path: gm in the morning, ge once the evening has
// a plan to close, gW at the end of a quiet week. A day holding none says so
// rather than standing bare.
func DailyGreeting(pulse WorkspacePulse, now time.Time) string {
	hour := now.Hour()
	salutation := "Good evening"
	switch {
	case hour < 5:
		salutation = "Working late"
	case hour < 12:
		salutation = "Good morning"
	case hour < 18:
		salutation = "Good afternoon"
	}
	// Before noon the move is the morning pass, which walks the overdue work and
	// the day's suggestions one card at a time.
	morning := hour >= 5 && hour < 12
	evening := hour >= 17
	switch {
	case morning && pulse.Overdue > 0:
		return salutation + " · overdue work first (gm Start the Day)"
	case pulse.Overdue > 0:
		return salutation + " · overdue work first (7 Due)"
	case pulse.Inbox > 0:
		return salutation + " · the Inbox wants sorting (I triage)"
	case evening && pulse.Today > 0:
		return salutation + " · the plan is ready to close (ge Close the Day)"
	case pulse.Today > 0:
		return salutation + " · the plan is ready (gt Today)"
	}
	if morning && pulse.Done == 0 {
		return salutation + " · nothing planned yet (gm Start the Day)"
	}
	if pulse.Done == 0 && hour < 15 {
		// An unplanned day is the one empty day with a ritual: Today offers what
		// is worth planning, and I walks it one card at a time.
		return salutation + " · nothing planned yet (gt, then I to plan the day)"
	}
	if weekWindsDown(now) {
		return salutation + " · a quiet moment to look back (gW Review the Week)"
	}
	return salutation + " · nothing is waiting on you"
}

// weekWindsDown is Friday afternoon through Sunday: the stretch a quiet day is
// worth spending on the week rather than on nothing.
func weekWindsDown(now time.Time) bool {
	switch now.Weekday() {
	case time.Saturday, time.Sunday:
		return true
	case time.Friday:
		return now.Hour() >= 12
	case time.Monday, time.Tuesday, time.Wednesday, time.Thursday:
		return false
	}
	return false
}
