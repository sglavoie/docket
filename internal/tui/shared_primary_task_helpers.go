package tui

import (
	"github.com/sglavoie/tt/internal/models"
)

// buildReparentValidParents returns project-local tasks that can accept task's
// complete subtree without creating a cycle or exceeding the lean hierarchy's
// six-level depth cap. Task types are migration-only metadata and do not
// constrain parent relationships.
func buildReparentValidParents(project *models.Project, task *models.Task) []*models.Task {
	if project == nil || task == nil {
		return nil
	}
	var parents []*models.Task
	excluded := taskDescendantIDs(task)
	subtreeHeight := taskSubtreeHeightForPicker(task)
	for _, t := range project.Tasks {
		if t.ID == task.ID || excluded[t.ID] {
			continue
		}
		if t.Level+1+subtreeHeight <= models.MaxNestingDepth {
			parents = append(parents, t)
		}
	}
	return parents
}

func taskSubtreeHeightForPicker(task *models.Task) int {
	if task == nil {
		return 0
	}
	height := 0
	for _, child := range task.Children {
		height = max(height, 1+taskSubtreeHeightForPicker(child))
	}
	return height
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
