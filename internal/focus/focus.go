package focus

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// FocusTask wraps a task with its parent project for cross-project display.
type FocusTask struct {
	Task    *models.Task
	Project *models.Project
}

// CollectActionableTasks filters actionable tasks from already-loaded projects.
// Tasks are considered actionable if they are not completed, not snoozed,
// and not blocked by incomplete dependencies.
func CollectActionableTasks(projects []*models.Project) []FocusTask {
	var tasks []FocusTask
	for _, project := range projects {
		for _, task := range project.Tasks {
			if task.Status.IsComplete() || task.IsSnoozed() || project.HasIncompleteDependencies(task) {
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
