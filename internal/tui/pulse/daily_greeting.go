package pulse

import (
	"fmt"
	"strings"
	"time"
)

// DailyGreeting is the line the first interactive launch of a local day
// opens with: a salutation for the hour, then what the day already holds —
// Today, overdue, Inbox — in the pulse's own spelling, each only when nonzero.
// A day holding none of them says so rather than standing bare.
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
	parts := make([]string, 0, 3)
	if pulse.Today > 0 {
		parts = append(parts, fmt.Sprintf("%d planned for Today", pulse.Today))
	}
	if pulse.Overdue > 0 {
		parts = append(parts, fmt.Sprintf("%d overdue", pulse.Overdue))
	}
	if pulse.Inbox > 0 {
		// The one count with a ritual behind it names its key: I opens triage.
		parts = append(parts, fmt.Sprintf("%d in Inbox (I triage)", pulse.Inbox))
	}
	if len(parts) == 0 {
		return salutation + " · nothing is waiting on you"
	}
	return salutation + " · " + strings.Join(parts, " · ")
}
