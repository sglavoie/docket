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

// rebaseTaskRestore applies an edit's inverse (or its redo) to the current
// project by task identity. Every touched task must still hold the complete
// expected persisted value, including its touch time. Other tasks and project
// metadata come from current. This deliberately has no field-level merge: a
// later edit to the same task always refuses, even on a different field.
//
// The entry's original images remain its intent on both stacks. Comparing them
// again on every restore preserves unrelated work through repeated undo/redo,
// later external edits, and restarts without changing the ledger format.
func rebaseTaskRestore(expectedBytes, desiredBytes, currentBytes []byte) ([]byte, bool) {
	expected, err := parseProjectDocumentContent(string(expectedBytes))
	if err != nil {
		return nil, false
	}
	desired, err := parseProjectDocumentContent(string(desiredBytes))
	if err != nil {
		return nil, false
	}
	// Creation, deletion, ordering, and project metadata operations still
	// require their exact images. The semantic path covers existing-task edits.
	if expected.ProjectName != desired.ProjectName ||
		!slices.EqualFunc(expected.Comments, desired.Comments, models.ProjectCommentEqual) ||
		!slices.EqualFunc(expected.Tasks, desired.Tasks, func(a, b *models.Task) bool { return a.ID == b.ID }) {
		return nil, false
	}
	current, err := parseProjectDocumentContent(string(currentBytes))
	if err != nil {
		return nil, false
	}
	currentByID := indexTasks(current.Tasks)
	replacements := make(map[string]*models.Task)
	for i, task := range expected.Tasks {
		replacement := desired.Tasks[i]
		if restoreTasksEqual(task, replacement) {
			continue
		}
		live := currentByID[task.ID]
		if live == nil || !restoreTasksEqual(task, live) {
			return nil, false
		}
		replacements[task.ID] = replacement
	}
	if len(replacements) == 0 {
		return nil, false
	}
	for i, task := range current.Tasks {
		if replacement := replacements[task.ID]; replacement != nil {
			current.Tasks[i] = replacement
		}
	}
	// Independent task edits can still compose an invalid hierarchy, such as
	// undoing a parent's reopening after a peer added an open child.
	if len(auditMergedDocument(current)) != 0 {
		return nil, false
	}
	rendered, err := current.WriteCurrent()
	return []byte(rendered), err == nil
}

func restoreTasksEqual(a, b *models.Task) bool {
	return a.UpdatedAt.Equal(b.UpdatedAt) && tasksEqual(a, b)
}
