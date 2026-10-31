package storage

import (
	"slices"

	"github.com/sglavoie/docket/internal/models"
)

func sameRestoreTaskOrder(left, right []*models.Task) bool {
	return slices.EqualFunc(left, right, func(a, b *models.Task) bool { return a.ID == b.ID })
}

// A status commit may stably partition finished roots after active ones. Admit
// exactly that ordering effect in either restore direction; a standalone or
// additional manual reorder still requires the exact document image.
func taskRestoreOrderCompatible(expected, desired *ProjectDocument) bool {
	if sameRestoreTaskOrder(expected.Tasks, desired.Tasks) {
		return true
	}
	if len(expected.Tasks) != len(desired.Tasks) {
		return false
	}
	desiredByID := indexTasks(desired.Tasks)
	boundary := false
	for _, task := range expected.Tasks {
		other := desiredByID[task.ID]
		if other == nil || !sameParent(task.ParentID, other.ParentID) {
			return false
		}
		boundary = boundary || task.IsComplete() != other.IsComplete()
	}
	return boundary && (completionPartitionMatches(expected, desired) || completionPartitionMatches(desired, expected))
}

func completionPartitionMatches(source, target *ProjectDocument) bool {
	targetByID := indexTasks(target.Tasks)
	project := &models.Project{}
	for _, task := range source.Tasks {
		project.Tasks = append(project.Tasks, cloneTask(targetByID[task.ID]))
	}
	project.ReorderRootTasksByCompletion()
	return sameRestoreTaskOrder(project.Tasks, target.Tasks)
}
