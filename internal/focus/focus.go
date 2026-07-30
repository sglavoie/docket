package focus

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
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
// reference time is not time.Now(), mirroring Task.IsSnoozed /
// Task.IsSnoozedAt.
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

func hasIncompleteDependencies(task *models.Task, project *models.Project, projects []*models.Project) bool {
	if task == nil || project == nil {
		return false
	}
	for _, depID := range task.DependsOn {
		ref := models.ParseTaskRef(depID)
		if ref.IsCrossProject() {
			dep := getTaskByRef(projects, ref)
			if dep == nil || !dep.IsComplete() {
				return true
			}
			continue
		}
		dep := getTaskByID(project, depID)
		if dep == nil || !dep.IsComplete() {
			return true
		}
	}
	return false
}

func getTaskByRef(projects []*models.Project, ref models.TaskRef) *models.Task {
	if ref.IsEmpty() {
		return nil
	}
	for _, project := range projects {
		if project == nil || project.ID != ref.ProjectID {
			continue
		}
		return getTaskByID(project, ref.TaskID)
	}
	return nil
}

func getTaskByID(project *models.Project, id string) *models.Task {
	if project == nil || id == "" {
		return nil
	}
	for _, task := range project.Tasks {
		if task != nil && task.ID == id {
			return task
		}
	}
	return nil
}
