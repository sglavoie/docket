package pulse

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

// ArrivalKind is why a task asks for attention now and did not before.
type ArrivalKind uint8

const (
	ArrivalCheckIn ArrivalKind = iota
	ArrivalDeadline
	ArrivalWake
)

// Arrival is one task that crossed into needing attention, and why. The two
// IDs are stored identities, so a host acting on the arrival later names the
// same task even after a rename or a renumbering.
type Arrival struct {
	Kind      ArrivalKind
	ProjectID string
	TaskID    string
	Title     string
	// Reviewable is whether acknowledging the task is allowed at now, so a
	// notification offers that action only where Docket would accept it.
	Reviewable bool
}

// Arrivals lists the tasks that crossed into needing attention between two
// instants, at most one reason per task. Each reason is the predicate the rows
// themselves answer, read at both ends of the window, so a notification can
// never name something the Outliner would not show as arrived: a check-in whose
// day came (and is not suppressed by a deferral), a timed deadline that passed,
// or a deferral that ended. A date-only deadline is not an arrival: its day is
// the Today lane's and the overdue count's, not a moment. Hidden work never
// arrives, since it is never owed attention.
func Arrivals(projects []*models.Project, since, now time.Time) []Arrival {
	var arrivals []Arrival
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil || task.IsTerminalState() || models.TaskEffectivelyHidden(project, task) {
				continue
			}
			arrival := Arrival{
				ProjectID:  project.ID,
				TaskID:     task.ID,
				Title:      task.Title,
				Reviewable: models.TaskAllowsExplicitReview(task, now),
			}
			switch {
			case task.CheckInIsDueAt(now) && !task.CheckInIsDueAt(since):
				arrival.Kind = ArrivalCheckIn
			case task.DueHasTime() && !task.IsDeferredAt(now) &&
				TaskIsOverdueAt(task, now) && !TaskIsOverdueAt(task, since):
				arrival.Kind = ArrivalDeadline
			case task.DeferredUntil != nil && task.IsDeferredAt(since) && !task.IsDeferredAt(now):
				arrival.Kind = ArrivalWake
			default:
				continue
			}
			arrivals = append(arrivals, arrival)
		}
	}
	return arrivals
}

// ArrivalNotification words a window's arrivals as one notification: a single
// arrival names its task, several are counted by reason.
func ArrivalNotification(arrivals []Arrival) (title, body string) {
	if len(arrivals) == 1 {
		a := arrivals[0]
		switch a.Kind {
		case ArrivalCheckIn:
			return "Docket · check-in", a.Title
		case ArrivalDeadline:
			return "Docket · deadline passed", a.Title
		default:
			return "Docket · deferral ended", a.Title
		}
	}
	var counts [3]int
	for _, a := range arrivals {
		counts[a.Kind]++
	}
	var parts []string
	if n := counts[ArrivalCheckIn]; n > 0 {
		parts = append(parts, plural(n, "check-in", "check-ins"))
	}
	if n := counts[ArrivalDeadline]; n > 0 {
		parts = append(parts, plural(n, "deadline passed", "deadlines passed"))
	}
	if n := counts[ArrivalWake]; n > 0 {
		parts = append(parts, plural(n, "deferral ended", "deferrals ended"))
	}
	return "Docket", strings.Join(parts, " · ")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// DesktopNotification is the rxvt-unicode notify escape (OSC 777), which
// carries a title as well as a body; Ghostty, and therefore Docket.app, presents it
// as a system notification. The title ends at the first ';', so one in it is
// replaced; control characters would end or corrupt the sequence anywhere.
func DesktopNotification(title, body string) string {
	clean := func(s string) string {
		return strings.Map(func(r rune) rune {
			if r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) {
				return ' '
			}
			return r
		}, s)
	}
	return "\x1b]777;notify;" + strings.ReplaceAll(clean(title), ";", ",") + ";" + clean(body) + "\x07"
}
