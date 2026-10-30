package storage

import (
	"path/filepath"
	"slices"

	"github.com/sglavoie/docket/internal/models"
)

// isProjectTaskImage limits semantic restores to existing project documents.
// Notes and attachments, even a file named tasks.md, retain byte preconditions.
func (l *WorkspaceLedger) isProjectTaskImage(image ArtifactImage) bool {
	if image.imageKind() != ArtifactImageKindFile || !image.BeforeExists || !image.AfterExists {
		return false
	}
	path := filepath.Clean(image.Path)
	if filepath.Base(path) != tasksFileName {
		return false
	}
	for _, root := range l.roots {
		if filepath.Dir(filepath.Dir(path)) == filepath.Join(root, "projects") {
			return true
		}
	}
	return false
}

// rebaseTaskRestore applies an edit or standalone creation's inverse (or redo) to the current
// project by task identity. Every touched task must still hold the complete
// expected persisted value, including its touch time. Other tasks and project
// metadata come from current. This deliberately has no field-level merge: a
// later edit to the same task always refuses, even on a different field.
//
// The entry's original images remain its intent on both stacks. Comparing them
// again on every restore preserves unrelated work through repeated undo/redo,
// later external edits, and restarts without changing the ledger format.
func rebaseTaskRestore(expectedBytes, desiredBytes, currentBytes []byte, redo, allowCreation bool) ([]byte, bool) {
	expected, err := parseProjectDocumentContent(string(expectedBytes))
	if err != nil {
		return nil, false
	}
	desired, err := parseProjectDocumentContent(string(desiredBytes))
	if err != nil {
		return nil, false
	}
	if expected.ProjectName != desired.ProjectName ||
		!slices.EqualFunc(expected.Comments, desired.Comments, models.ProjectCommentEqual) {
		return nil, false
	}
	current, err := parseProjectDocumentContent(string(currentBytes))
	if err != nil {
		return nil, false
	}
	if slices.EqualFunc(expected.Tasks, desired.Tasks, func(a, b *models.Task) bool { return a.ID == b.ID }) {
		if !rebaseEditedTasks(expected, desired, current) {
			return nil, false
		}
	} else if !allowCreation || !rebaseCreatedTask(expected, desired, current, redo) {
		return nil, false
	}
	// Independent changes can still compose an invalid hierarchy or reuse a
	// created task's project-local ShortID while it is on the redo stack.
	if len(auditMergedDocument(current)) != 0 {
		return nil, false
	}
	rendered, err := current.WriteCurrent()
	return []byte(rendered), err == nil
}

func rebaseEditedTasks(expected, desired, current *ProjectDocument) bool {
	currentByID := indexTasks(current.Tasks)
	replacements := make(map[string]*models.Task)
	for i, task := range expected.Tasks {
		replacement := desired.Tasks[i]
		if restoreTasksEqual(task, replacement) {
			continue
		}
		live := currentByID[task.ID]
		if live == nil || !restoreTasksEqual(task, live) {
			return false
		}
		replacements[task.ID] = replacement
	}
	if len(replacements) == 0 {
		return false
	}
	for i, task := range current.Tasks {
		if replacement := replacements[task.ID]; replacement != nil {
			current.Tasks[i] = replacement
		}
	}
	return true
}

func restoreTasksEqual(a, b *models.Task) bool {
	return a.UpdatedAt.Equal(b.UpdatedAt) && tasksEqual(a, b)
}
