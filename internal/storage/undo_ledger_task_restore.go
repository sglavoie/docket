package storage

import (
	"path/filepath"
	"reflect"
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

// rebaseTaskRestore applies task edits, project-comment changes, or a standalone
// task addition/removal to the current project by identity. Title, priority,
// context, resume-note and lifecycle edits check their changed fields or coupled
// field groups; other task edits and comment changes require their complete
// expected persisted values.
// Untouched values and the project name come from current.
//
// The entry's original images remain its intent on both stacks. Comparing them
// again on every restore preserves unrelated work through repeated undo/redo,
// later external edits, and restarts without changing the ledger format.
func rebaseTaskRestore(expectedBytes, desiredBytes, currentBytes []byte, allowMembershipChange bool) ([]byte, bool) {
	expected, err := parseProjectDocumentContent(string(expectedBytes))
	if err != nil {
		return nil, false
	}
	desired, err := parseProjectDocumentContent(string(desiredBytes))
	if err != nil {
		return nil, false
	}
	if expected.ProjectName != desired.ProjectName {
		return nil, false
	}
	current, err := parseProjectDocumentContent(string(currentBytes))
	if err != nil {
		return nil, false
	}
	commentsChanged := !slices.EqualFunc(expected.Comments, desired.Comments, models.ProjectCommentEqual)
	if taskRestoreOrderCompatible(expected, desired) {
		tasksChanged := !slices.EqualFunc(expected.Tasks, desired.Tasks, restoreTasksEqual)
		if tasksChanged && !rebaseEditedTasks(expected, desired, current) {
			return nil, false
		}
		if commentsChanged {
			if !rebaseProjectCommentsRestore(expected, desired, current) {
				return nil, false
			}
		} else if !tasksChanged {
			return nil, false
		}
	} else if commentsChanged || !allowMembershipChange || !rebaseStandaloneTask(expected, desired, current) {
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
	desiredByID := indexTasks(desired.Tasks)
	replacements := make(map[string]*models.Task)
	for _, task := range expected.Tasks {
		replacement := desiredByID[task.ID]
		if restoreTasksEqual(task, replacement) {
			continue
		}
		live := currentByID[task.ID]
		if live == nil {
			return false
		}
		if !restoreTasksEqual(task, live) {
			var ok bool
			replacement, ok = rebaseTaskFieldRestore(task, replacement, live)
			if !ok {
				return false
			}
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
	if !sameRestoreTaskOrder(expected.Tasks, desired.Tasks) {
		// Completion partitions roots and their descendants. Preserve the
		// current order inside each partition rather than replaying an old
		// project-wide sequence over incoming additions and ordering changes.
		project := &models.Project{Tasks: current.Tasks}
		project.ReorderRootTasksByCompletion()
		current.Tasks = project.Tasks
	}
	return true
}

// rebaseTaskFieldRestore accepts independent scalar edits and lifecycle changes
// with their deferral and check-in effects. Every changed field or coupled group
// must still hold its expected value, even if current already holds the desired
// one. This is a conditional inverse, not a coalescing forward merge. Standalone
// schedules, hierarchy, and collections retain whole-task checks.
func rebaseTaskFieldRestore(expected, desired, current *models.Task) (*models.Task, bool) {
	lifecycle := expected.WorkflowState != desired.WorkflowState ||
		!expected.StateChangedAt.Equal(desired.StateChangedAt) ||
		!sameOptionalInstant(expected.CompletedAt, desired.CompletedAt)
	checkIn := lifecycle && !checkInUnitOf(expected).equals(checkInUnitOf(desired))
	merged := cloneTask(current)
	ev, dv, cv := reflect.ValueOf(expected).Elem(), reflect.ValueOf(desired).Elem(), reflect.ValueOf(current).Elem()
	mv := reflect.ValueOf(merged).Elem()
	changed := false
	for _, field := range mergeableTaskFields {
		e, d := ev.Field(field.index).Interface(), dv.Field(field.index).Interface()
		// Lifecycle owns its clock, completion data and deferral cleanup even
		// when an individual member did not change. A changed check-in is the
		// same three-field unit used by sync, including review history.
		if (lifecycle && lifecycleRestoreField(field.jsonName)) || (checkIn && field.checkInUnit) {
			if !field.equal(e, cv.Field(field.index).Interface()) {
				return nil, false
			}
			mv.Field(field.index).Set(dv.Field(field.index))
			changed = changed || !field.equal(e, d)
			continue
		}
		if field.equal(e, d) {
			continue
		}
		switch field.jsonName {
		case "title", "priority", "context", "resume_note":
			if !field.equal(e, cv.Field(field.index).Interface()) {
				return nil, false
			}
		default:
			return nil, false
		}
		mv.Field(field.index).Set(dv.Field(field.index))
		changed = true
	}
	// A timestamp-only entry has no independent field intent to restore. For a
	// rebased edit, keep the latest retained touch rather than erase peer history.
	if desired.UpdatedAt.After(merged.UpdatedAt) {
		merged.UpdatedAt = desired.UpdatedAt
	}
	return merged, changed
}

func lifecycleRestoreField(name string) bool {
	switch name {
	case "workflow_state", "state_changed_at", "completed_at", "deferred_until", "defer_count":
		return true
	default:
		return false
	}
}

func restoreTasksEqual(a, b *models.Task) bool {
	return a.UpdatedAt.Equal(b.UpdatedAt) && tasksEqual(a, b)
}
