package tui

import (
	"github.com/sglavoie/tt/internal/models"
)

// buildReparentValidParents returns tasks that are valid parents for reparenting task.
// Excludes the task itself, its descendants, and tasks that can't parent the task's type.
func buildReparentValidParents(project *models.Project, task *models.Task) []*models.Task {
	if project == nil || task == nil {
		return nil
	}
	var parents []*models.Task
	excluded := taskDescendantIDs(task)
	for _, t := range project.Tasks {
		if t.ID == task.ID || excluded[t.ID] {
			continue
		}
		if t.CanAddChild(task.Type) {
			parents = append(parents, t)
		}
	}
	return parents
}

// taskDescendantIDs returns the set of IDs of all descendants of task.
func taskDescendantIDs(task *models.Task) map[string]bool {
	ids := make(map[string]bool)
	for _, child := range task.Children {
		ids[child.ID] = true
		for id := range taskDescendantIDs(child) {
			ids[id] = true
		}
	}
	return ids
}
