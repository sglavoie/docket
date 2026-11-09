package app

import (
	"sort"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/util"
)

const completionActivityWindowDays = 30

// CompletionHistoryWeeks is how far back History reaches, in whole weeks: half
// a year, which is what an activity grid can show at a glance.
const CompletionHistoryWeeks = 26

// CompletionActivity is the read-only completion history derived from one
// revision-consistent workspace snapshot. DailyCompletions contains only days
// with activity; MedianPerDay still includes every zero-count day in the
// 30-local-calendar-day window. A caller can distinguish no history from a
// zero median with HasHistory.
type CompletionActivity struct {
	DailyCompletions map[time.Time]int
	// History is the Done count by local day over the last
	// CompletionHistoryWeeks weeks, today included, holding only days with
	// activity. It is read by the activity grid alone: every median, best day,
	// and HasHistory stays on the 30-day window above.
	History         map[time.Time]int
	AsOf            time.Time
	medianPerDay    float64
	completedToday  int
	activeDayMedian float64
	bestPriorDay    int
}

// IsCurrentLocalDay reports whether the snapshot was computed on the same
// calendar day as now in now's location. A zero AsOf is never current.
func (a CompletionActivity) IsCurrentLocalDay(now time.Time) bool {
	if a.AsOf.IsZero() {
		return false
	}
	asOf := a.AsOf.In(now.Location())
	return asOf.Year() == now.Year() && asOf.Month() == now.Month() && asOf.Day() == now.Day()
}

// HasHistory reports whether the activity window contains any completion.
func (a CompletionActivity) HasHistory() bool {
	return len(a.DailyCompletions) > 0
}

// MedianPerDay reports the median Done-task count across all 30 local-calendar
// days in the activity window, including days with zero completions.
func (a CompletionActivity) MedianPerDay() float64 {
	return a.medianPerDay
}

// ActiveDayMedian reports the median Done-task count across the window's days
// before today that finished anything, or 0 when none did. It is the honest
// reference for an occasional user, whose all-days median is 0: what a day
// that gets worked on usually closes.
func (a CompletionActivity) ActiveDayMedian() float64 {
	return a.activeDayMedian
}

// BestPriorDay reports the largest Done-task count on any window day before
// today, or 0 when none finished anything.
func (a CompletionActivity) BestPriorDay() int {
	return a.bestPriorDay
}

// CompletedToday reports the Done-task count in the current local calendar day.
func (a CompletionActivity) CompletedToday() int {
	return a.completedToday
}

// CompletionActivity derives completion activity from exactly one immutable
// repository snapshot. Activity means successfully Done work: Canceled tasks
// do not contribute even though they also carry CompletedAt. The project
// population mirrors config.RuntimeState.TaskWorkspaceProjects by excluding
// archived and parked projects. A median is used instead of a trimmed mean so
// isolated high-output days cannot dominate the reference-class rate.
func (s *Service) CompletionActivity() (CompletionActivity, error) {
	if source, ok := s.repo.(CompletionActivitySource); ok {
		projects, err := source.CompletionActivityProjects()
		if err != nil {
			return CompletionActivity{}, err
		}
		return completionActivityFromProjects(projects, s.clock.Now()), nil
	}
	snapshot, err := s.repo.Snapshot()
	if err != nil {
		return CompletionActivity{}, err
	}
	return completionActivityFromSnapshot(snapshot, s.clock.Now()), nil
}

// CompletionActivitySource is an optional repository capability: a read-only
// load of the task workspace — archived and parked projects already excluded —
// cheap enough to repeat after every commit. A full Snapshot re-reads and
// re-parses every project, and the count is refreshed on the interface's own
// goroutine, so a repository that can serve unchanged projects from a cache
// offers this instead. The projects are only read, never mutated.
type CompletionActivitySource interface {
	CompletionActivityProjects() ([]*models.Project, error)
}

func completionActivityFromSnapshot(snapshot Snapshot, now time.Time) CompletionActivity {
	var projects []*models.Project
	if snapshot != nil {
		for _, project := range snapshot.Projects() {
			if project == nil || project.Archived || models.ProjectTagsParked(project.Tags) {
				continue
			}
			projects = append(projects, project)
		}
	}
	return completionActivityFromProjects(projects, now)
}

// completionActivityFromProjects counts over an already-selected population;
// choosing it is the caller's job, so a cached project's stale catalog metadata
// is never consulted here. Hidden work is out of the pace as it is out of every
// count beside it.
func completionActivityFromProjects(projects []*models.Project, now time.Time) CompletionActivity {
	calendarAnchor := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
	activityDays := make([]time.Time, 0, completionActivityWindowDays)
	activityDaySet := make(map[time.Time]struct{}, completionActivityWindowDays)
	for dayOffset := completionActivityWindowDays - 1; dayOffset >= 0; dayOffset-- {
		day := util.StartOfDay(calendarAnchor.AddDate(0, 0, -dayOffset))
		activityDays = append(activityDays, day)
		activityDaySet[day] = struct{}{}
	}
	today := activityDays[len(activityDays)-1]
	daily := make(map[time.Time]int)
	historyStart := util.StartOfDay(calendarAnchor.AddDate(0, 0, -(CompletionHistoryWeeks*7 - 1)))
	history := make(map[time.Time]int)

	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil || task.WorkflowState != models.StateDone || task.CompletedAt == nil ||
				models.TaskEffectivelyHidden(project, task) {
				continue
			}
			completedDay := util.StartOfDay(task.CompletedAt.In(now.Location()))
			if !completedDay.Before(historyStart) && !completedDay.After(today) {
				history[completedDay]++
			}
			if _, ok := activityDaySet[completedDay]; !ok {
				continue
			}
			daily[completedDay]++
		}
	}

	counts := make([]int, 0, completionActivityWindowDays)
	active := make([]int, 0, completionActivityWindowDays)
	best := 0
	for _, day := range activityDays {
		counts = append(counts, daily[day])
		if day.Equal(today) || daily[day] == 0 {
			continue
		}
		active = append(active, daily[day])
		best = max(best, daily[day])
	}
	median := medianOfCounts(counts)

	return CompletionActivity{
		DailyCompletions: daily,
		History:          history,
		AsOf:             now,
		medianPerDay:     median,
		completedToday:   daily[today],
		activeDayMedian:  medianOfCounts(active),
		bestPriorDay:     best,
	}
}

// medianOfCounts is the median of counts, averaging the middle pair of an even
// set, or 0 for an empty one. It sorts counts in place.
func medianOfCounts(counts []int) float64 {
	if len(counts) == 0 {
		return 0
	}
	sort.Ints(counts)
	middle := len(counts) / 2
	if len(counts)%2 == 1 {
		return float64(counts[middle])
	}
	return float64(counts[middle-1]+counts[middle]) / 2
}
