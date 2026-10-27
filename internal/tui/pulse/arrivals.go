package pulse

import (
	"cmp"
	"fmt"
	"slices"
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
			if arrival, ok := arrivalBetween(project, task, since, now); ok {
				arrivals = append(arrivals, arrival)
			}
		}
	}
	return arrivals
}

// arrivalBetween is Arrivals for one task: the reason it crossed into needing
// attention in (since, now], if it did. UpcomingArrivals asks the same question
// at each instant a task could arrive, so a scheduled reminder and a live
// arrival can never disagree about whether, or why, a task arrives.
func arrivalBetween(project *models.Project, task *models.Task, since, now time.Time) (Arrival, bool) {
	if task == nil || task.IsTerminalState() || models.TaskEffectivelyHidden(project, task) {
		return Arrival{}, false
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
		return Arrival{}, false
	}
	return arrival, true
}

// UpcomingLimit bounds how many upcoming arrivals a schedule names, and
// UpcomingHorizon how far ahead it looks. A host books each one with the
// system ahead of time, and macOS keeps at most 64 pending per app; the
// schedule is recomputed at least once a local day, so the horizon only has
// to outlast the longest the host may go without hearing a newer one.
const (
	UpcomingLimit   = 48
	UpcomingHorizon = 8 * 24 * time.Hour
)

// UpcomingArrival is an Arrival that has not happened yet: At is the instant
// it will, as long as nothing about the task changes first.
type UpcomingArrival struct {
	Arrival
	At time.Time
}

// upcomingProbe is how far past a candidate instant the arrival predicate is
// read: a timed deadline is overdue only after its instant, never at it.
const upcomingProbe = time.Millisecond

// UpcomingArrivals lists what Arrivals will report after now and within
// horizon, earliest first, at most limit of them. Each candidate instant — a
// check-in's local day starting, a timed deadline, a deferral ending — is
// tested with the very predicate Arrivals answers, over the window that
// instant opens, so a deferral that suppresses a check-in or a deadline is
// honored exactly as a tick would honor it, and the task arrives at its wake
// instead. Reviewable is read at the arrival, not now.
func UpcomingArrivals(projects []*models.Project, now time.Time, horizon time.Duration, limit int) []UpcomingArrival {
	end := now.Add(horizon)
	var upcoming []UpcomingArrival
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil || task.IsTerminalState() || models.TaskEffectivelyHidden(project, task) {
				continue
			}
			for _, at := range arrivalCandidates(task, now) {
				if !at.After(now) || at.After(end) {
					continue
				}
				if arrival, ok := arrivalBetween(project, task, at.Add(-upcomingProbe), at.Add(upcomingProbe)); ok {
					upcoming = append(upcoming, UpcomingArrival{Arrival: arrival, At: at})
				}
			}
		}
	}
	slices.SortStableFunc(upcoming, func(a, b UpcomingArrival) int {
		if c := a.At.Compare(b.At); c != 0 {
			return c
		}
		if c := cmp.Compare(a.ProjectID, b.ProjectID); c != 0 {
			return c
		}
		return cmp.Compare(a.TaskID, b.TaskID)
	})
	return upcoming[:min(len(upcoming), limit)]
}

// arrivalCandidates are the instants a task's attention could change: where
// its check-in's calendar day starts in now's zone, its timed deadline, and
// its deferral's end. A candidate is only a question; arrivalBetween answers it.
func arrivalCandidates(task *models.Task, now time.Time) []time.Time {
	var candidates []time.Time
	if next := task.NextCheckIn; next != nil {
		day := time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, now.Location())
		candidates = append(candidates, day)
	}
	if task.DueHasTime() {
		candidates = append(candidates, *task.DueDate)
	}
	if task.DeferredUntil != nil {
		candidates = append(candidates, *task.DeferredUntil)
	}
	return candidates
}

// EqualUpcoming reports whether two schedules name the same arrivals at the
// same instants, so an unchanged one is never sent again.
func EqualUpcoming(a, b []UpcomingArrival) bool {
	return slices.EqualFunc(a, b, func(x, y UpcomingArrival) bool {
		return x.Arrival == y.Arrival && x.At.Equal(y.At)
	})
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
