package storage

import (
	"slices"

	"github.com/sglavoie/docket/internal/models"
)

// rebaseStandaloneTask restores an entry that added or deleted one root task,
// leaving every other task and its order unchanged. The caller restricts this
// to a single project artifact, so coupled changes retain exact checks.
func rebaseStandaloneTask(expected, desired, current *ProjectDocument) bool {
	added, index := standaloneAddedTask(expected, desired)
	if added != nil {
		return insertRestoredTask(desired, current, added, index)
	}
	removed, _ := standaloneAddedTask(desired, expected)
	if removed == nil {
		return false
	}
	return removeRestoredTask(current, removed)
}

func removeRestoredTask(current *ProjectDocument, removed *models.Task) bool {
	index := slices.IndexFunc(current.Tasks, func(task *models.Task) bool { return task.ID == removed.ID })
	if index < 0 || !restoreTasksEqual(current.Tasks[index], removed) {
		return false
	}
	for _, task := range current.Tasks {
		if task.ParentID != nil && *task.ParentID == removed.ID {
			return false
		}
	}
	current.Tasks = slices.Delete(current.Tasks, index, index+1)
	return true
}

func insertRestoredTask(original, current *ProjectDocument, added *models.Task, addedIndex int) bool {
	currentByID := indexTasks(current.Tasks)
	if currentByID[added.ID] != nil {
		return false
	}
	// Insert before the next surviving original root. This retains current
	// sibling order and keeps new descendants with their parent. With no such
	// anchor, append at the end, including when every original task is gone.
	insertAt := len(current.Tasks)
	for _, task := range original.Tasks[addedIndex+1:] {
		if live := currentByID[task.ID]; live != nil && live.ParentID == nil {
			insertAt = slices.IndexFunc(current.Tasks, func(candidate *models.Task) bool { return candidate.ID == live.ID })
			break
		}
	}
	current.Tasks = slices.Insert(current.Tasks, insertAt, added)
	return true
}

func standaloneAddedTask(before, after *ProjectDocument) (*models.Task, int) {
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

// Unreadable retained documents are outside the standalone task policy;
// their ordinary exact-image preflight remains authoritative.
func standaloneAddedTaskInImages(beforeBytes, afterBytes []byte) *models.Task {
	before, err := parseProjectDocumentContent(string(beforeBytes))
	if err != nil {
		return nil
	}
	after, err := parseProjectDocumentContent(string(afterBytes))
	if err != nil {
		return nil
	}
	created, _ := standaloneAddedTask(before, after)
	return created
}
