package storage

import (
	"slices"

	"github.com/sglavoie/docket/internal/models"
)

// rebaseCreatedTask handles only an entry that originally added one root task,
// leaving every existing task and its order unchanged. The caller restricts
// this to a single project artifact, so coupled changes retain exact checks.
func rebaseCreatedTask(expected, desired, current *ProjectDocument, redo bool) bool {
	before, after := desired, expected
	if redo {
		before, after = expected, desired
	}
	created, createdIndex := standaloneCreatedTask(before, after)
	if created == nil {
		return false
	}
	index := slices.IndexFunc(current.Tasks, func(task *models.Task) bool { return task.ID == created.ID })
	if !redo {
		if index < 0 || !restoreTasksEqual(current.Tasks[index], created) {
			return false
		}
		for _, task := range current.Tasks {
			if task.ParentID != nil && *task.ParentID == created.ID {
				return false
			}
		}
		current.Tasks = slices.Delete(current.Tasks, index, index+1)
		return true
	}
	if index >= 0 {
		return false
	}
	// Insert before the next surviving original root. This retains current
	// sibling order and keeps new descendants with their parent. With no such
	// anchor, append at the end, including when every original task is gone.
	insertAt := len(current.Tasks)
	currentByID := indexTasks(current.Tasks)
	for _, task := range after.Tasks[createdIndex+1:] {
		if live := currentByID[task.ID]; live != nil && live.ParentID == nil {
			insertAt = slices.IndexFunc(current.Tasks, func(candidate *models.Task) bool { return candidate.ID == live.ID })
			break
		}
	}
	current.Tasks = slices.Insert(current.Tasks, insertAt, created)
	return true
}

func standaloneCreatedTask(before, after *ProjectDocument) (*models.Task, int) {
	if before.ProjectName != after.ProjectName || !slices.EqualFunc(before.Comments, after.Comments, models.ProjectCommentEqual) {
		return nil, 0
	}
	if len(after.Tasks) != len(before.Tasks)+1 {
		return nil, 0
	}
	var created *models.Task
	createdIndex, next := 0, 0
	for i, task := range after.Tasks {
		if next < len(before.Tasks) && task.ID == before.Tasks[next].ID {
			if !restoreTasksEqual(task, before.Tasks[next]) {
				return nil, 0
			}
			next++
			continue
		}
		if created != nil {
			return nil, 0
		}
		created, createdIndex = task, i
	}
	if created == nil || next != len(before.Tasks) || created.ParentID != nil || len(created.Attachments) != 0 {
		return nil, 0
	}
	return created, createdIndex
}

// Unreadable retained documents are outside the standalone creation policy;
// their ordinary exact-image preflight remains authoritative.
func standaloneCreatedTaskInImages(beforeBytes, afterBytes []byte) *models.Task {
	before, err := parseProjectDocumentContent(string(beforeBytes))
	if err != nil {
		return nil
	}
	after, err := parseProjectDocumentContent(string(afterBytes))
	if err != nil {
		return nil
	}
	created, _ := standaloneCreatedTask(before, after)
	return created
}
