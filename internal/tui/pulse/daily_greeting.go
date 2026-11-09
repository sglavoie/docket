package pulse

import (
	"fmt"
	"time"
)

// DailyGreeting is the line the first interactive launch of a local day
// opens with: a salutation for the hour, then the one move the day asks for
// first. The pulse already counts Today, overdue, and Inbox on the title row,
// so the greeting does not recount them; it chooses among them — overdue work
// before an untriaged Inbox before the plan — and names only that, with the
// key that starts it where one exists. A day holding none says so rather than
// standing bare.
func DailyGreeting(pulse WorkspacePulse, now time.Time) string {
	salutation := "Good evening"
	switch hour := now.Hour(); {
	case hour < 5:
		salutation = "Working late"
	case hour < 12:
		salutation = "Good morning"
	case hour < 18:
		salutation = "Good afternoon"
	}
	// Before noon the move is the morning pass, which walks the overdue work and
	// the day's suggestions one card at a time.
	morning := now.Hour() >= 5 && now.Hour() < 12
	switch {
	case morning && pulse.Overdue > 0:
		return salutation + " · " + fmt.Sprintf("start with the %d overdue (Task → Start the Day)", pulse.Overdue)
	case pulse.Overdue > 0:
		return salutation + " · " + fmt.Sprintf("start with the %d overdue (7 Due)", pulse.Overdue)
	case pulse.Inbox > 0:
		return salutation + " · " + fmt.Sprintf("%d in Inbox to sort (I triage)", pulse.Inbox)
	case pulse.Today > 0:
		return salutation + " · " + fmt.Sprintf("%s planned and ready (gt Today)", plural(pulse.Today, "task", "tasks"))
	}
	if morning && pulse.Done == 0 {
		return salutation + " · nothing planned yet (Task → Start the Day)"
	}
	if pulse.Done == 0 && now.Hour() < 15 {
		// An unplanned morning is the one empty day with a ritual: Today offers
		// what is worth planning, and I walks it one card at a time.
		return salutation + " · nothing planned yet (gt, then I to plan the day)"
	}
	return salutation + " · nothing is waiting on you"
}
