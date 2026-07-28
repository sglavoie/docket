package tui

import (
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// PriorityGroup is the urgency grouping used by the Outliner's optional
// cross-project Priority mode.
type PriorityGroup int

const (
	PriorityGroupOverdue PriorityGroup = iota
	PriorityGroupToday
	PriorityGroupThisWeek
	PriorityGroupLater
	PriorityGroupBacklog
	PriorityGroupWaitingOn
)

// PriorityTask carries the urgency metadata rendered by the Outliner's
// cross-project Priority mode.
type PriorityTask struct {
	Task         *models.Task
	UrgencyScore int
	ParentTitle  string
	RelativeDue  string
}

// IsEligibleForPriority reports whether a task is a candidate for the priority
// view. Completed and snoozed tasks are excluded; parent/container tasks are
// kept so they remain visible in priority listings.
func IsEligibleForPriority(task *models.Task) bool {
	if task.IsTerminalState() {
		return false
	}
	if task.IsSnoozed() {
		return false
	}
	return true
}

// IsPriorityBlocked reports whether a task should be grouped under
// Blocked/Waiting in the priority view. A task is blocked either because it
// sits in the waiting state or because it has unresolved dependencies in the
// supplied project.
func IsPriorityBlocked(project *models.Project, task *models.Task) bool {
	if task.IsInState(models.StateWaiting) {
		return true
	}
	return project.HasIncompleteDependencies(task)
}

// CategorizeByDueDate maps a task's due date to its priority group.
func CategorizeByDueDate(task *models.Task) PriorityGroup {
	if task.DueDate == nil {
		return PriorityGroupBacklog
	}

	today := util.StartOfDay(time.Now())
	dueDay := util.StartOfDay(*task.DueDate)

	if dueDay.Before(today) {
		return PriorityGroupOverdue
	}
	if dueDay.Equal(today) {
		return PriorityGroupToday
	}
	if dueDay.Before(today.AddDate(0, 0, 7)) {
		return PriorityGroupThisWeek
	}
	return PriorityGroupLater
}

// ComputeUrgencyScore calculates the urgency score for a task. Higher scores
// are more urgent. The scoring is shared between single-project and
// cross-project priority modes. inFocusQueue is the task's explicit Focus Queue
// membership.
func ComputeUrgencyScore(task *models.Task, inFocusQueue bool) int {
	score := (6 - models.EffectivePriority(task.Priority)) * 1000

	if task.Type == models.TaskTypeBug {
		score += 100
	}

	if task.DueDate != nil {
		daysUntil := util.DaysBetween(time.Now(), *task.DueDate)
		score += (-daysUntil) * 50
	}

	if task.IsInState(models.StateInProgress) {
		score += 500
	}

	if task.IsInState(models.StateReview) {
		score += 250
	}

	if inFocusQueue {
		score += 200
	}

	return score
}

// SortPriorityTasksByUrgency stably sorts tasks by descending urgency score.
// Stability matters because UrgencyScore is a coarse integer and ties are
// common; an unstable sort would flip order between reloads.
func SortPriorityTasksByUrgency(tasks []*PriorityTask) {
	sort.SliceStable(tasks, func(i, j int) bool {
		return tasks[i].UrgencyScore > tasks[j].UrgencyScore
	})
}
