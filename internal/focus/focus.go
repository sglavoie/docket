package focus

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// FocusTask wraps a task with its parent project for cross-project display.
type FocusTask struct {
	Task       *models.Task
	Project    *models.Project
	TreePrefix string
}

// CollectActionableTasks filters actionable tasks from already-loaded projects.
// Tasks are considered actionable if they are not completed, not snoozed,
// and not blocked by incomplete dependencies. Nil projects and nil tasks in
// the input are skipped so callers can pass loosely-assembled slices without
// pre-filtering them.
func CollectActionableTasks(projects []*models.Project) []FocusTask {
	return CollectActionableTasksAt(projects, time.Now())
}

// CollectActionableTasksAt is CollectActionableTasks with an explicit reference
// time for the snooze check. Use this instead of CollectActionableTasks when the
// reference time is not time.Now() (e.g., the recommendation engine's
// deterministic clock), mirroring Task.IsSnoozed / Task.IsSnoozedAt.
func CollectActionableTasksAt(projects []*models.Project, now time.Time) []FocusTask {
	var tasks []FocusTask
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			// Actionability is state-derived: terminal tasks are done, waiting
			// tasks are gated on external input, snoozed tasks are hidden, and
			// dependency-blocked tasks are not yet ready. Exclusion now keys off
			// the canonical StateWaiting:
			// once v2 is the sole writer a blocked task persists as the waiting
			// state (todo + Waiting section), so keying off status alone would let
			// every reloaded blocked task leak back into the actionable set. This
			// also folds the legacy Waiting section into the same exclusion,
			// completing the workflow migration.
			if task.IsTerminalState() || task.IsInState(models.StateWaiting) || task.IsSnoozedAt(now) || hasIncompleteDependencies(task, project, projects) {
				continue
			}
			tasks = append(tasks, FocusTask{Task: task, Project: project})
		}
	}
	return tasks
}

// FormatRelativeDue formats a due date relative to today as plain text.
func FormatRelativeDue(dueDate *time.Time) string {
	if dueDate == nil {
		return ""
	}

	now := time.Now()

	daysUntil := util.DaysBetween(now, *dueDate)

	switch {
	case daysUntil < -1:
		return fmt.Sprintf("%d days ago", -daysUntil)
	case daysUntil == -1:
		return "yesterday"
	case daysUntil == 0:
		return "today"
	case daysUntil == 1:
		return "tomorrow"
	case daysUntil < 7:
		return dueDate.Format("Mon")
	default:
		if dueDate.Year() != now.Year() {
			return dueDate.Format("Jan 2 '06")
		}
		return dueDate.Format("Jan 2")
	}
}
