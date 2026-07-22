package tui

import (
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// PriorityGroup is the urgency grouping used by the Outliner's optional
// cross-project priority lens.
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
// cross-project priority lens.
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
	score := (6 - task.Priority) * 1000

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

// crossPriorityCollapseKeyPrefix marks a section-header collapse key as
// belonging to the cross-project Priority mode. The prefix exists so the
// shared section header renderer and toggle helpers can route collapse-state
// lookups to OutlinerModel.crossPriorityCollapsedGroups instead of the
// persisted m.collapsedSections map.
const crossPriorityCollapseKeyPrefix = "_xpp:"

// priorityGroupSlug returns a stable, lowercase identifier for a priority
// group. Used to build collapse-key sentinels and for tests.
func priorityGroupSlug(g PriorityGroup) string {
	switch g {
	case PriorityGroupOverdue:
		return "overdue"
	case PriorityGroupToday:
		return "today"
	case PriorityGroupThisWeek:
		return "this_week"
	case PriorityGroupLater:
		return "later"
	case PriorityGroupBacklog:
		return "backlog"
	case PriorityGroupWaitingOn:
		return "blocked"
	}
	return "unknown"
}

// priorityGroupHeaderLabel returns the human-readable label shown in a
// cross-project Priority mode section header. Uppercase mirrors the
// single-project Priority View header style.
func priorityGroupHeaderLabel(g PriorityGroup) string {
	switch g {
	case PriorityGroupOverdue:
		return "OVERDUE"
	case PriorityGroupToday:
		return "DUE TODAY"
	case PriorityGroupThisWeek:
		return "THIS WEEK"
	case PriorityGroupLater:
		return "LATER"
	case PriorityGroupBacklog:
		return "BACKLOG"
	case PriorityGroupWaitingOn:
		return "BLOCKED / WAITING"
	}
	return ""
}

// crossPriorityCollapseKey builds the sentinel collapse key for a priority
// group in cross-project Priority mode.
func crossPriorityCollapseKey(g PriorityGroup) string {
	return crossPriorityCollapseKeyPrefix + priorityGroupSlug(g)
}

// priorityGroupFromCollapseKey decodes a sentinel collapse key produced by
// crossPriorityCollapseKey. Returns ok=false for any non-priority key.
func priorityGroupFromCollapseKey(key string) (PriorityGroup, bool) {
	if len(key) <= len(crossPriorityCollapseKeyPrefix) ||
		key[:len(crossPriorityCollapseKeyPrefix)] != crossPriorityCollapseKeyPrefix {
		return 0, false
	}
	slug := key[len(crossPriorityCollapseKeyPrefix):]
	for _, g := range CrossPriorityGroupOrder() {
		if priorityGroupSlug(g) == slug {
			return g, true
		}
	}
	return 0, false
}

// CrossPriorityGroupOrder returns the canonical display order for cross-project
// Priority mode group headers.
func CrossPriorityGroupOrder() []PriorityGroup {
	return []PriorityGroup{
		PriorityGroupOverdue,
		PriorityGroupToday,
		PriorityGroupThisWeek,
		PriorityGroupLater,
		PriorityGroupBacklog,
		PriorityGroupWaitingOn,
	}
}
