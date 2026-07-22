package app

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// SetDueDate sets or clears a task's due date. A nil due clears the date (and
// its clock component); HasTime marks a meaningful time-of-day on a non-nil due.
func (s *Service) SetDueDate(taskID string, due *time.Time, hasTime bool) (TaskResult, error) {
	return s.mutateTask(taskID, "Set Due Date", "", func(ref TaskRef) error {
		if due == nil {
			ref.Task.DueDate = nil
			ref.Task.HasTime = false
		} else {
			d := *due
			ref.Task.DueDate = &d
			ref.Task.HasTime = hasTime
		}
		ref.Task.UpdatedAt = s.clock.Now()
		return nil
	})
}

func (s *Service) SetDueDateByLocator(locator TaskLocator, due *time.Time, hasTime bool) (TaskResult, error) {
	return s.mutateTaskByLocator(locator, "Set Due Date", "", func(ref TaskRef) error {
		if due == nil {
			ref.Task.DueDate = nil
			ref.Task.HasTime = false
		} else {
			d := *due
			ref.Task.DueDate = &d
			ref.Task.HasTime = hasTime
		}
		ref.Task.UpdatedAt = s.clock.Now()
		return nil
	})
}

// SetPriority sets a task's priority. The value must be within the valid
// 1..PriorityMax range.
func (s *Service) SetPriority(taskID string, priority int) (TaskResult, error) {
	return s.mutateTask(taskID, "Set Priority", "", func(ref TaskRef) error {
		if err := models.ValidatePriority(priority, false); err != nil {
			return invalidInput("priority", err.Error())
		}
		ref.Task.Priority = priority
		ref.Task.UpdatedAt = s.clock.Now()
		return nil
	})
}

func (s *Service) SetPriorityByLocator(locator TaskLocator, priority int) (TaskResult, error) {
	return s.mutateTaskByLocator(locator, "Set Priority", "", func(ref TaskRef) error {
		if err := models.ValidatePriority(priority, false); err != nil {
			return invalidInput("priority", err.Error())
		}
		ref.Task.Priority = priority
		ref.Task.UpdatedAt = s.clock.Now()
		return nil
	})
}

// AddTag adds a normalized tag to a task. A malformed tag is rejected with
// ErrInvalidInput rather than silently dropped.
func (s *Service) AddTag(taskID, tag string) (TaskResult, error) {
	return s.mutateTask(taskID, "Add Tag", tag, func(ref TaskRef) error {
		normalized := models.NormalizeTag(tag)
		if err := models.ValidateTag(normalized); err != nil {
			return invalidInput("tag", err.Error())
		}
		ref.Task.AddTag(normalized)
		return nil
	})
}

func (s *Service) AddTagByLocator(locator TaskLocator, tag string) (TaskResult, error) {
	return s.mutateTaskByLocator(locator, "Add Tag", tag, func(ref TaskRef) error {
		normalized := models.NormalizeTag(tag)
		if err := models.ValidateTag(normalized); err != nil {
			return invalidInput("tag", err.Error())
		}
		ref.Task.AddTag(normalized)
		return nil
	})
}

// RemoveTag removes a tag from a task. Removing a tag the task does not carry is
// a no-op that still commits the (unchanged) project.
func (s *Service) RemoveTag(taskID, tag string) (TaskResult, error) {
	return s.mutateTask(taskID, "Remove Tag", tag, func(ref TaskRef) error {
		normalized := models.NormalizeTag(tag)
		if err := models.ValidateTag(normalized); err != nil {
			return invalidInput("tag", err.Error())
		}
		ref.Task.RemoveTag(normalized)
		return nil
	})
}

func (s *Service) RemoveTagByLocator(locator TaskLocator, tag string) (TaskResult, error) {
	return s.mutateTaskByLocator(locator, "Remove Tag", tag, func(ref TaskRef) error {
		normalized := models.NormalizeTag(tag)
		if err := models.ValidateTag(normalized); err != nil {
			return invalidInput("tag", err.Error())
		}
		ref.Task.RemoveTag(normalized)
		return nil
	})
}

// SetPinned pins or unpins a task.
func (s *Service) SetPinned(taskID string, pinned bool) (TaskResult, error) {
	action := "Unpin Task"
	if pinned {
		action = "Pin Task"
	}
	return s.mutateTask(taskID, action, "", func(ref TaskRef) error {
		if pinned {
			ref.Task.Pin()
		} else {
			ref.Task.Unpin()
		}
		return nil
	})
}

func (s *Service) SetPinnedByLocator(locator TaskLocator, pinned bool) (TaskResult, error) {
	action := "Unpin Task"
	if pinned {
		action = "Pin Task"
	}
	return s.mutateTaskByLocator(locator, action, "", func(ref TaskRef) error {
		if pinned {
			ref.Task.Pin()
		} else {
			ref.Task.Unpin()
		}
		return nil
	})
}

// PinnedReorderResult reports the projects a pinned-task reorder committed, so
// an adapter can reload exactly those projects and rebuild its pinned view.
type PinnedReorderResult struct {
	// TouchedProjectIDs lists the committed projects (one entry when both tasks
	// share a project, two otherwise).
	TouchedProjectIDs []string
}

// ReorderPinnedTasks swaps the PinnedAt timestamps of two pinned tasks so their
// relative order in a PinnedAt-sorted list flips, committing every touched
// project as ONE transaction. Both tasks must resolve and both must be pinned;
// when their timestamps are identical the second is nudged one second forward so
// the swap produces a strict ordering instead of a no-op. A same-project pair
// commits once; a cross-project pair commits both projects atomically.
func (s *Service) ReorderPinnedTasks(taskAID, taskBID string) (PinnedReorderResult, error) {
	return s.reorderPinnedTasks(taskAID, TaskLocator{}, taskBID, TaskLocator{})
}

func (s *Service) ReorderPinnedTasksByLocator(taskA, taskB TaskLocator) (PinnedReorderResult, error) {
	return s.reorderPinnedTasks("", taskA, "", taskB)
}

func (s *Service) reorderPinnedTasks(taskAID string, taskA TaskLocator, taskBID string, taskB TaskLocator) (PinnedReorderResult, error) {
	refA, snap, revA, err := s.loadTaskSelection(taskAID, taskA)
	if err != nil {
		return PinnedReorderResult{}, err
	}
	refB, err := resolveTaskSelection(snap, taskBID, taskB)
	if err != nil {
		return PinnedReorderResult{}, err
	}
	if !refA.Task.Pinned || !refB.Task.Pinned {
		return PinnedReorderResult{}, invalidInput("pin", "both tasks must be pinned to reorder")
	}

	var aTime, bTime time.Time
	if refA.Task.PinnedAt != nil {
		aTime = *refA.Task.PinnedAt
	}
	if refB.Task.PinnedAt != nil {
		bTime = *refB.Task.PinnedAt
	}
	if aTime.Equal(bTime) {
		bTime = bTime.Add(time.Second)
	}
	newA, newB := bTime, aTime
	refA.Task.PinnedAt = &newA
	refB.Task.PinnedAt = &newB

	commits := []projectCommit{{project: refA.Project, expectedRevision: revA, base: refA.Base}}
	touched := []string{refA.Project.ID}
	if refB.Project.ID != refA.Project.ID {
		commits = append(commits, projectCommit{project: refB.Project, expectedRevision: snap.Revision(refB.Project.ID), base: snap.Document(refB.Project.ID)})
		touched = append(touched, refB.Project.ID)
	}
	undoContext := fmt.Sprintf("#%d <-> #%d", refA.Task.ShortID, refB.Task.ShortID)
	if _, err := s.commitProjects(commits, "Reorder Pinned", undoContext); err != nil {
		return PinnedReorderResult{}, err
	}
	return PinnedReorderResult{TouchedProjectIDs: touched}, nil
}

// Snooze hides a task until the given date, or clears the snooze when until is
// nil. Snoozing a terminal or in-progress task is rejected: hiding a completed
// task is meaningless and hiding an actively worked-on one is contradictory.
// These guards live here so every adapter (CLI and TUI) enforces them uniformly
// rather than replicating the check at each command edge.
func (s *Service) Snooze(taskID string, until *time.Time) (TaskResult, error) {
	return s.mutateTask(taskID, "Snooze Task", "", func(ref TaskRef) error {
		return s.applySnooze(ref.Task, until, true)
	})
}

func (s *Service) SnoozeByLocator(locator TaskLocator, until *time.Time) (TaskResult, error) {
	return s.mutateTaskByLocator(locator, "Snooze Task", "", func(ref TaskRef) error {
		return s.applySnooze(ref.Task, until, true)
	})
}

func (s *Service) applySnooze(task *models.Task, until *time.Time, guarded bool) error {
	if until == nil {
		task.ClearSnooze()
		return nil
	}
	if guarded && task.IsTerminalState() {
		return invalidInput("snooze", fmt.Sprintf("cannot snooze a %s task", task.State()))
	}
	if guarded && task.IsInState(models.StateInProgress) {
		return invalidInput("snooze", "cannot snooze an in-progress task (change state first)")
	}
	u := *until
	task.SnoozedUntil = &u
	task.UpdatedAt = s.clock.Now()
	return nil
}

// Park stores a resume note on a task, preserving any previous note as a
// timestamped comment. Parking a terminal task is rejected — recording where you
// left off makes no sense once the task is done or canceled.
func (s *Service) Park(taskID, note string) (TaskResult, error) {
	return s.mutateTask(taskID, "Park Task", "", func(ref TaskRef) error {
		return s.parkTask(ref.Task, note)
	})
}

func (s *Service) ParkByLocator(locator TaskLocator, note string) (TaskResult, error) {
	return s.mutateTaskByLocator(locator, "Park Task", "", func(ref TaskRef) error {
		return s.parkTask(ref.Task, note)
	})
}

func (s *Service) parkTask(task *models.Task, note string) error {
	if task.IsTerminalState() {
		return invalidInput("park", fmt.Sprintf("cannot park a %s task", task.State()))
	}
	task.Park(note)
	return nil
}

// ParkWithWorkflow parks a task through the same guarded mutation as Park and
// then runs the configured contextual or stateless workflow cascade.
func (s *Service) ParkWithWorkflow(taskID, note string, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	opts = withDefaultWorkflowUndoAction(opts, "Park Task")
	return s.mutateWithWorkflow(taskID, opts, func(ref TaskRef, _ *WorkspaceResolver) error {
		return s.parkTask(ref.Task, note)
	})
}

func (s *Service) ParkWithWorkflowByLocator(locator TaskLocator, note string, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	opts = withDefaultWorkflowUndoAction(opts, "Park Task")
	return s.mutateWithWorkflowByLocator(locator, opts, func(ref TaskRef, _ *WorkspaceResolver) error {
		return s.parkTask(ref.Task, note)
	})
}

// Unpark clears a task's parked state. The note is discarded rather than rolled
// into a timestamped comment, matching the established CLI and TUI unpark
// semantics (both call Unpark(false)); an explicit re-park is what preserves the
// prior note as a comment.
func (s *Service) Unpark(taskID string) (TaskResult, error) {
	return s.mutateTask(taskID, "Unpark Task", "", func(ref TaskRef) error {
		return unparkTask(ref.Task)
	})
}

func (s *Service) UnparkByLocator(locator TaskLocator) (TaskResult, error) {
	return s.mutateTaskByLocator(locator, "Unpark Task", "", func(ref TaskRef) error {
		return unparkTask(ref.Task)
	})
}

func unparkTask(task *models.Task) error {
	task.Unpark(false)
	return nil
}

// UnparkWithWorkflow clears the parked state and runs the workflow cascade. An
// already-unparked task preserves the legacy CLI no-op behavior.
func (s *Service) UnparkWithWorkflow(taskID string, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	opts = withDefaultWorkflowUndoAction(opts, "Unpark Task")
	return s.mutateWithWorkflowGuarded(taskID, opts, func(ref TaskRef) bool {
		return !ref.Task.IsParked()
	}, func(ref TaskRef, _ *WorkspaceResolver) error {
		return unparkTask(ref.Task)
	})
}

func (s *Service) UnparkWithWorkflowByLocator(locator TaskLocator, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	opts = withDefaultWorkflowUndoAction(opts, "Unpark Task")
	return s.mutateWithWorkflowGuardedByLocator(locator, opts, func(ref TaskRef) bool {
		return !ref.Task.IsParked()
	}, func(ref TaskRef, _ *WorkspaceResolver) error {
		return unparkTask(ref.Task)
	})
}

// ToggleDimmed flips a task's dimmed flag, muting or restoring it in every view.
// Dimming is purely visual and leaves workflow state, section, and Focus Queue
// membership unchanged.
func (s *Service) ToggleDimmed(taskID string) (TaskResult, error) {
	return s.mutateTask(taskID, "Toggle Dim", "", func(ref TaskRef) error {
		return toggleDimmedTask(ref.Task)
	})
}

func (s *Service) ToggleDimmedByLocator(locator TaskLocator) (TaskResult, error) {
	return s.mutateTaskByLocator(locator, "Toggle Dim", "", func(ref TaskRef) error {
		return toggleDimmedTask(ref.Task)
	})
}

func toggleDimmedTask(task *models.Task) error {
	task.ToggleDimmed()
	return nil
}

// Undim clears a task's dimmed flag, restoring normal rendering. Undimming an
// already-undimmed task still commits an (unchanged) project, matching the other
// idempotent attribute mutations.
func (s *Service) Undim(taskID string) (TaskResult, error) {
	return s.mutateTask(taskID, "Undim Task", "", func(ref TaskRef) error {
		return undimTask(ref.Task)
	})
}

func (s *Service) UndimByLocator(locator TaskLocator) (TaskResult, error) {
	return s.mutateTaskByLocator(locator, "Undim Task", "", func(ref TaskRef) error {
		return undimTask(ref.Task)
	})
}

func undimTask(task *models.Task) error {
	task.Undim()
	return nil
}

// UndimWithWorkflow clears dimming and runs the workflow cascade. An
// already-undimmed task is returned without evaluating rules or committing.
func (s *Service) UndimWithWorkflow(taskID string, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	opts = withDefaultWorkflowUndoAction(opts, "Undim Task")
	return s.mutateWithWorkflowGuarded(taskID, opts, func(ref TaskRef) bool {
		return !ref.Task.Dimmed
	}, func(ref TaskRef, _ *WorkspaceResolver) error {
		return undimTask(ref.Task)
	})
}

func (s *Service) UndimWithWorkflowByLocator(locator TaskLocator, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	opts = withDefaultWorkflowUndoAction(opts, "Undim Task")
	return s.mutateWithWorkflowGuardedByLocator(locator, opts, func(ref TaskRef) bool {
		return !ref.Task.Dimmed
	}, func(ref TaskRef, _ *WorkspaceResolver) error {
		return undimTask(ref.Task)
	})
}

// SetSection moves a task to the workflow state a GTD section name maps to,
// committing through the ledger (Task.SetSection performs the state move). A
// non-GTD name is silently ignored by the model, matching the retired section
// rules. Custom sections and stages are retired under the workflow-state model.
//
// Because SetSection changes the derived workflow state, it folds the same
// Today/Next exclusivity reconciliation a Transition does into the commit: a task
// moved into StateNext leaves the Daily Plan (Today) and a task moved out of
// StateNext drops its stale Next order entry, both in the same transaction, so a
// section move can never leave a task in both horizons or with a stale order entry.
func (s *Service) SetSection(taskID, section string) (TaskResult, error) {
	return s.setSection(taskID, TaskLocator{}, section)
}

func (s *Service) SetSectionByLocator(locator TaskLocator, section string) (TaskResult, error) {
	return s.setSection("", locator, section)
}

func (s *Service) setSection(taskID string, locator TaskLocator, section string) (TaskResult, error) {
	ref, _, revision, err := s.loadTaskSelection(taskID, locator)
	if err != nil {
		return TaskResult{}, err
	}
	ref.Task.SetSection(section)
	artifacts, err := s.transitionPlanningArtifacts(ref, s.defaultedPlanningArtifacts(nil))
	if err != nil {
		return TaskResult{}, err
	}
	return s.commitProjectWithArtifacts(ref, revision, artifacts, "Set Section", section)
}

// ApplyTypeAttrs applies type-specific attributes (bug/idea/task/story/epic
// fields) to a task through the shared models implementation, so CLI flag input
// and typed capture stay in parity. Invalid attribute values are rejected before
// any commit, leaving the task untouched.
func (s *Service) ApplyTypeAttrs(taskID string, vals models.TypeAttrValues) (TaskResult, error) {
	return s.mutateTask(taskID, "Set Attributes", "", func(ref TaskRef) error {
		return applyTypeAttrs(ref.Task, vals)
	})
}

func (s *Service) ApplyTypeAttrsByLocator(locator TaskLocator, vals models.TypeAttrValues) (TaskResult, error) {
	return s.mutateTaskByLocator(locator, "Set Attributes", "", func(ref TaskRef) error {
		return applyTypeAttrs(ref.Task, vals)
	})
}

func applyTypeAttrs(task *models.Task, vals models.TypeAttrValues) error {
	return models.ApplyTypeAttrValues(task, vals)
}

// ApplyTypeAttrsWithWorkflow applies validated type-specific attributes and
// runs the workflow cascade in one transaction.
func (s *Service) ApplyTypeAttrsWithWorkflow(taskID string, vals models.TypeAttrValues, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	opts = withDefaultWorkflowUndoAction(opts, "Set Attributes")
	return s.mutateWithWorkflow(taskID, opts, func(ref TaskRef, _ *WorkspaceResolver) error {
		return applyTypeAttrs(ref.Task, vals)
	})
}

func (s *Service) ApplyTypeAttrsWithWorkflowByLocator(locator TaskLocator, vals models.TypeAttrValues, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	opts = withDefaultWorkflowUndoAction(opts, "Set Attributes")
	return s.mutateWithWorkflowByLocator(locator, opts, func(ref TaskRef, _ *WorkspaceResolver) error {
		return applyTypeAttrs(ref.Task, vals)
	})
}

// ClearTypeAttrs removes all type-specific attributes from a task.
func (s *Service) ClearTypeAttrs(taskID string) (TaskResult, error) {
	return s.mutateTask(taskID, "Clear Attributes", "", func(ref TaskRef) error {
		return clearTypeAttrs(ref.Task)
	})
}

func (s *Service) ClearTypeAttrsByLocator(locator TaskLocator) (TaskResult, error) {
	return s.mutateTaskByLocator(locator, "Clear Attributes", "", func(ref TaskRef) error {
		return clearTypeAttrs(ref.Task)
	})
}

func clearTypeAttrs(task *models.Task) error {
	task.ClearTypeAttrs()
	return nil
}

// ClearTypeAttrsWithWorkflow removes all type-specific attributes and runs the
// configured workflow cascade.
func (s *Service) ClearTypeAttrsWithWorkflow(taskID string, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	opts = withDefaultWorkflowUndoAction(opts, "Clear Attributes")
	return s.mutateWithWorkflow(taskID, opts, func(ref TaskRef, _ *WorkspaceResolver) error {
		return clearTypeAttrs(ref.Task)
	})
}

func (s *Service) ClearTypeAttrsWithWorkflowByLocator(locator TaskLocator, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	opts = withDefaultWorkflowUndoAction(opts, "Clear Attributes")
	return s.mutateWithWorkflowByLocator(locator, opts, func(ref TaskRef, _ *WorkspaceResolver) error {
		return clearTypeAttrs(ref.Task)
	})
}
