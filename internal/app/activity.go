package app

import (
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

const completionActivityWindowDays = 30

// CompletionActivity is the read-only completion history derived from one
// revision-consistent workspace snapshot. DailyCompletions contains only days
// with activity; MedianPerDay still includes every zero-count day in the
// 30-local-calendar-day window. A caller can distinguish no history from a
// zero median with HasHistory.
type CompletionActivity struct {
	DailyCompletions map[time.Time]int
	medianPerDay     float64
	completedToday   int
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
	snapshot, err := s.repo.Snapshot()
	if err != nil {
		return CompletionActivity{}, err
	}
	return completionActivityFromSnapshot(snapshot, s.clock.Now()), nil
}

func completionActivityFromSnapshot(snapshot Snapshot, now time.Time) CompletionActivity {
	today := util.StartOfDay(now)
	windowStart := today.AddDate(0, 0, -(completionActivityWindowDays - 1))
	windowEnd := today.AddDate(0, 0, 1)
	daily := make(map[time.Time]int)

	if snapshot != nil {
		for _, project := range snapshot.Projects() {
			if project == nil || project.Archived || models.ProjectTagsParked(project.Tags) {
				continue
			}
			for _, task := range project.Tasks {
				if task == nil || task.WorkflowState != models.StateDone || task.CompletedAt == nil {
					continue
				}
				completedDay := util.StartOfDay(task.CompletedAt.In(now.Location()))
				if completedDay.Before(windowStart) || !completedDay.Before(windowEnd) {
					continue
				}
				daily[completedDay]++
			}
		}
	}

	counts := make([]int, 0, completionActivityWindowDays)
	for day := windowStart; day.Before(windowEnd); day = day.AddDate(0, 0, 1) {
		counts = append(counts, daily[day])
	}
	sort.Ints(counts)
	middle := completionActivityWindowDays / 2
	median := float64(counts[middle-1]+counts[middle]) / 2

	return CompletionActivity{
		DailyCompletions: daily,
		medianPerDay:     median,
		completedToday:   daily[today],
	}
}
