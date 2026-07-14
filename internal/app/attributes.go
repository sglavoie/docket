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
	refA, snap, revA, err := s.loadTask(taskAID)
	if err != nil {
		return PinnedReorderResult{}, err
	}
	refB, ok := snap.Lookup(taskBID)
	if !ok {
		return PinnedReorderResult{}, fmt.Errorf("%w: %s", ErrTaskNotFound, taskBID)
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
		if until == nil {
			ref.Task.ClearSnooze()
			return nil
		}
		if ref.Task.IsTerminalState() {
			return invalidInput("snooze", fmt.Sprintf("cannot snooze a %s task", ref.Task.State()))
		}
		if ref.Task.IsInState(models.StateInProgress) {
			return invalidInput("snooze", "cannot snooze an in-progress task (change state first)")
		}
		u := *until
		ref.Task.SnoozedUntil = &u
		ref.Task.UpdatedAt = s.clock.Now()
		return nil
	})
}

// Park stores a resume note on a task, preserving any previous note as a
// timestamped comment. Parking a terminal task is rejected — recording where you
// left off makes no sense once the task is done or canceled.
func (s *Service) Park(taskID, note string) (TaskResult, error) {
	return s.mutateTask(taskID, "Park Task", "", func(ref TaskRef) error {
		if ref.Task.IsTerminalState() {
			return invalidInput("park", fmt.Sprintf("cannot park a %s task", ref.Task.State()))
		}
		ref.Task.Park(note)
		return nil
	})
}

// Unpark clears a task's parked state. The note is discarded rather than rolled
// into a timestamped comment, matching the established CLI and TUI unpark
// semantics (both call Unpark(false)); an explicit re-park is what preserves the
// prior note as a comment.
func (s *Service) Unpark(taskID string) (TaskResult, error) {
	return s.mutateTask(taskID, "Unpark Task", "", func(ref TaskRef) error {
		ref.Task.Unpark(false)
		return nil
	})
}

// ToggleDimmed flips a task's dimmed flag, muting or restoring it in every view.
// Dimming also clears the active/focus flag, since a dimmed task cannot be the
// focus; that coupling lives in the model's ToggleDimmed.
func (s *Service) ToggleDimmed(taskID string) (TaskResult, error) {
	return s.mutateTask(taskID, "Toggle Dim", "", func(ref TaskRef) error {
		ref.Task.ToggleDimmed()
		return nil
	})
}

// Undim clears a task's dimmed flag, restoring normal rendering. Undimming an
// already-undimmed task still commits an (unchanged) project, matching the other
// idempotent attribute mutations.
func (s *Service) Undim(taskID string) (TaskResult, error) {
	return s.mutateTask(taskID, "Undim Task", "", func(ref TaskRef) error {
		ref.Task.Undim()
		return nil
	})
}

// SetSection moves a task to the workflow state a GTD section name maps to,
// committing through the ledger (Task.SetSection performs the state move). A
// non-GTD name is silently ignored by the model, matching the retired section
// rules. Custom sections and stages are retired under the workflow-state model.
func (s *Service) SetSection(taskID, section string) (TaskResult, error) {
	return s.mutateTask(taskID, "Set Section", section, func(ref TaskRef) error {
		ref.Task.SetSection(section)
		return nil
	})
}

// ApplyTypeAttrs applies type-specific attributes (bug/idea/task/story/epic
// fields) to a task through the shared models implementation, so CLI flag input
// and typed capture stay in parity. Invalid attribute values are rejected before
// any commit, leaving the task untouched.
func (s *Service) ApplyTypeAttrs(taskID string, vals models.TypeAttrValues) (TaskResult, error) {
	return s.mutateTask(taskID, "Set Attributes", "", func(ref TaskRef) error {
		return models.ApplyTypeAttrValues(ref.Task, vals)
	})
}

// ClearTypeAttrs removes all type-specific attributes from a task.
func (s *Service) ClearTypeAttrs(taskID string) (TaskResult, error) {
	return s.mutateTask(taskID, "Clear Attributes", "", func(ref TaskRef) error {
		ref.Task.ClearTypeAttrs()
		return nil
	})
}
