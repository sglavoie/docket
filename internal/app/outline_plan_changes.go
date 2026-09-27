package app

import (
	"fmt"
	"slices"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// partitionOutlineChanges splits the applied buffer into the tasks it created
// and the existing tasks it really changed, so the reported count is the number
// of edits the buffer made rather than the number of lines it held.
func partitionOutlineChanges(baseProject *models.Project, prepared []preparedOutlineLine) (created, updated []*models.Task) {
	baseRanks := outlineBaseRanks(baseProject, prepared)
	rank := 0
	for _, item := range prepared {
		if item.creates() {
			created = append(created, item.task)
			continue
		}
		if outlineTaskChanged(baseProject.GetTaskByID(item.task.ID), item.task, baseRanks[item.task.ID], rank) {
			updated = append(updated, item.task)
		}
		rank++
	}
	return created, updated
}

// outlineBaseRanks ranks the buffer's existing tasks by their prior order among
// each other, so a pure reorder counts as a change while an unrelated insertion
// elsewhere in the project does not.
func outlineBaseRanks(baseProject *models.Project, prepared []preparedOutlineLine) map[string]int {
	buffered := make(map[string]bool, len(prepared))
	for _, item := range prepared {
		if !item.creates() {
			buffered[item.task.ID] = true
		}
	}
	ranks := make(map[string]int, len(buffered))
	rank := 0
	for _, task := range baseProject.Tasks {
		if task == nil || !buffered[task.ID] {
			continue
		}
		ranks[task.ID] = rank
		rank++
	}
	return ranks
}

func outlineTaskChanged(base, task *models.Task, baseRank, rank int) bool {
	if base == nil {
		return true
	}
	return base.Title != task.Title ||
		base.Priority != task.Priority ||
		base.Context != task.Context ||
		base.State() != task.State() ||
		base.Pinned != task.Pinned ||
		base.Review != task.Review ||
		baseRank != rank ||
		!slices.Equal(base.Tags, task.Tags) ||
		!models.TaskReferencesEqual(base.Dependencies, task.Dependencies) ||
		!sameOptionalID(base.ParentID, task.ParentID) ||
		!sameOptionalDate(base.DueDate, task.DueDate)
}

func sameOptionalID(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func sameOptionalDate(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

// outlineDeletions resolves the tasks the buffer removed, in dump order, and
// refuses anything the request cannot account for.
//
// The absent set is derived here rather than trusted from DeleteTaskIDs, and the
// two must agree exactly. That equality is the whole guarantee: it proves the
// list the adapter counted and showed is the list this transaction removes.
func outlineDeletions(project *models.Project, req OutlinePlanRequest, prepared []preparedOutlineLine) ([]*models.Task, error) {
	buffered := make(map[string]int, len(prepared))
	for _, item := range prepared {
		if !item.creates() {
			buffered[item.task.ID] = item.line.SourceLine
		}
	}
	confirmed := make(map[string]bool, len(req.DeleteTaskIDs))
	for _, taskID := range req.DeleteTaskIDs {
		if taskID == "" {
			return nil, invalidInput("delete", "task id is required")
		}
		if project.GetTaskByID(taskID) == nil {
			return nil, fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
		}
		if line, held := buffered[taskID]; held {
			return nil, invalidInput("delete", fmt.Sprintf("task %s is on line %d and cannot also be deleted", taskID, line))
		}
		confirmed[taskID] = true
	}

	seen := make(map[string]bool, len(req.ScopeTaskIDs))
	deleted := make([]*models.Task, 0, len(confirmed))
	for _, taskID := range req.ScopeTaskIDs {
		if taskID == "" {
			return nil, invalidInput("scope", "task id is required")
		}
		task := project.GetTaskByID(taskID)
		if task == nil {
			return nil, fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
		}
		if _, held := buffered[taskID]; held || seen[taskID] {
			continue
		}
		seen[taskID] = true
		if !confirmed[taskID] {
			return nil, invalidInput("delete", fmt.Sprintf("task %s is missing from the buffer but was not confirmed for deletion", taskID))
		}
		deleted = append(deleted, task)
	}
	for taskID := range confirmed {
		if !seen[taskID] {
			return nil, invalidInput("delete", fmt.Sprintf("task %s was confirmed for deletion but the buffer still holds it", taskID))
		}
	}
	if len(deleted) > 0 && !req.ConfirmDeletions {
		return nil, fmt.Errorf("%w: deleting %s the buffer no longer holds", ErrDestructiveConfirmationRequired, util.Count(len(deleted), "task"))
	}
	return deleted, nil
}

// removeOutlineTasks drops the deleted tasks from the project and returns the
// side effects the same transaction has to carry. It runs before the buffer's
// own parentage is applied, so a survivor whose parent line vanished is
// re-parented by its own indentation rather than by the removal.
func removeOutlineTasks(project *models.Project, deleted []*models.Task) ([]storage.AttachmentOp, []TaskLocator) {
	if len(deleted) == 0 {
		return nil, nil
	}
	removed := make(map[string]bool, len(deleted))
	ops := make([]storage.AttachmentOp, 0, len(deleted))
	locators := make([]TaskLocator, 0, len(deleted))
	for _, task := range deleted {
		removed[task.ID] = true
		ops = append(ops, storage.AttachmentOp{
			Kind:          storage.AttachmentDelete,
			Task:          task,
			SourceProject: project,
		})
		locators = append(locators, TaskLocator{ProjectID: project.ID, TaskID: task.ID})
	}
	project.Tasks = filterOutTasks(project.Tasks, removed)
	return ops, locators
}

// outlineReferenceMaintenance repairs the incoming references of the tasks one
// buffer deleted. The scope project is named as already committing, so it is
// rewritten in place rather than gaining a second changeset for the document the
// plan is already writing.
func (s *Service) outlineReferenceMaintenance(snap Snapshot, project *models.Project, removed []TaskLocator) (referenceMaintenancePass, error) {
	if len(removed) == 0 {
		return referenceMaintenancePass{}, nil
	}
	maintenance := TaskReferenceMaintenance{RemovedTasks: make(map[models.TaskReference]bool, len(removed))}
	for _, locator := range removed {
		maintenance.RemovedTasks[locator.Reference()] = true
	}
	return s.maintainTaskReferences(snap, maintenance, map[string]bool{project.ID: true})
}
