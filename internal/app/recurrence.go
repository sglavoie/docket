package app

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// RecurrenceResult is the outcome of a recurrence mutation. Task is the
// presentation-neutral task outcome shared with every other mutation; Pattern
// and DueDate carry the post-mutation recurrence display fields a CLI or TUI
// adapter renders (the pattern string after a set change, the next due
// date after a skip/defer) without reaching back into the domain model.
type RecurrenceResult struct {
	// Task is the standard task mutation result.
	Task TaskResult
	// Pattern is the task's recurrence pattern string after the mutation ("" when
	// the task no longer recurs).
	Pattern string
	// DueDate is the task's due date after the mutation.
	DueDate *time.Time
}

// mutateRecurrence is the shared runner for every single-task recurrence
// mutation. It resolves the task, applies mutate (which enforces the recurrence
// domain rules and may return a typed error), commits one ChangeSet, and reads
// the post-mutation recurrence display fields off the mutated task so adapters can
// render the new pattern/due date. A mutate error aborts before any commit.
func (s *Service) mutateRecurrence(taskID, undoAction string, undoContext func(*models.Task) string, mutate func(*models.Task) error) (RecurrenceResult, error) {
	return s.mutateRecurrenceSelection(taskID, TaskLocator{}, undoAction, undoContext, mutate)
}

func (s *Service) mutateRecurrenceByLocator(locator TaskLocator, undoAction string, undoContext func(*models.Task) string, mutate func(*models.Task) error) (RecurrenceResult, error) {
	return s.mutateRecurrenceSelection("", locator, undoAction, undoContext, mutate)
}

func (s *Service) mutateRecurrenceSelection(taskID string, locator TaskLocator, undoAction string, undoContext func(*models.Task) string, mutate func(*models.Task) error) (RecurrenceResult, error) {
	ref, snap, revision, err := s.loadTaskSelection(taskID, locator)
	if err != nil {
		return RecurrenceResult{}, err
	}
	before, err := cloneSnapshot(snap)
	if err != nil {
		return RecurrenceResult{}, fmt.Errorf("capture recurrence rollback state: %w", err)
	}
	keepMutation := false
	defer func() {
		if !keepMutation {
			restoreSnapshotProjects(snap, before)
		}
	}()
	if mutateErr := mutate(ref.Task); mutateErr != nil {
		return RecurrenceResult{}, mutateErr
	}
	res, err := s.commitProject(ref, revision, undoAction, undoContext(ref.Task))
	if err != nil {
		return RecurrenceResult{}, err
	}
	keepMutation = true
	return RecurrenceResult{
		Task:    res,
		Pattern: ref.Task.Recurrence.String(),
		DueDate: ref.Task.DueDate,
	}, nil
}

// SetRecurrence sets (or replaces) a task's recurrence pattern. A nil pattern is
// rejected — clearing goes through ClearRecurrence — and a pattern that fails the
// model's validation is reported as invalid input before any commit.
func (s *Service) SetRecurrence(taskID string, pattern *models.RecurrencePattern) (RecurrenceResult, error) {
	if pattern == nil {
		return RecurrenceResult{}, invalidInput("recurrence", "pattern is required")
	}
	context := pattern.String()
	return s.mutateRecurrence(taskID, "Set Recurrence",
		func(*models.Task) string { return context },
		func(t *models.Task) error {
			if err := t.SetRecurrence(pattern); err != nil {
				return invalidInput("recurrence", err.Error())
			}
			return nil
		})
}

func (s *Service) SetRecurrenceByLocator(locator TaskLocator, pattern *models.RecurrencePattern) (RecurrenceResult, error) {
	if pattern == nil {
		return RecurrenceResult{}, invalidInput("recurrence", "pattern is required")
	}
	context := pattern.String()
	return s.mutateRecurrenceByLocator(locator, "Set Recurrence",
		func(*models.Task) string { return context },
		func(t *models.Task) error {
			if err := t.SetRecurrence(pattern); err != nil {
				return invalidInput("recurrence", err.Error())
			}
			return nil
		})
}

// ClearRecurrence removes a task's recurrence pattern. Clearing a task that is
// not recurring is a no-op that still commits the (unchanged) project, matching
// the idempotent attribute mutations; adapters that want to avoid recording a
// no-op undo entry precheck the loaded task first.
func (s *Service) ClearRecurrence(taskID string) (RecurrenceResult, error) {
	return s.mutateRecurrence(taskID, "Clear Recurrence",
		func(t *models.Task) string { return fmt.Sprintf("#%d", t.ShortID) },
		func(t *models.Task) error {
			t.ClearRecurrence()
			return nil
		})
}

func (s *Service) ClearRecurrenceByLocator(locator TaskLocator) (RecurrenceResult, error) {
	return s.mutateRecurrenceByLocator(locator, "Clear Recurrence",
		func(t *models.Task) string { return fmt.Sprintf("#%d", t.ShortID) },
		func(t *models.Task) error {
			t.ClearRecurrence()
			return nil
		})
}

// SkipRecurrence is a temporary compatibility action that advances a recurring
// task to its next interval without completing it.
func (s *Service) SkipRecurrence(taskID string) (RecurrenceResult, error) {
	return s.mutateRecurrence(taskID, "Skip Recurrence",
		func(t *models.Task) string { return fmt.Sprintf("#%d", t.ShortID) },
		func(t *models.Task) error {
			if t.Recurrence == nil {
				return invalidInput("recurrence", "task is not recurring")
			}
			t.SkipRecurrence()
			return nil
		})
}

func (s *Service) SkipRecurrenceByLocator(locator TaskLocator) (RecurrenceResult, error) {
	return s.mutateRecurrenceByLocator(locator, "Skip Recurrence",
		func(t *models.Task) string { return fmt.Sprintf("#%d", t.ShortID) },
		func(t *models.Task) error {
			if t.Recurrence == nil {
				return invalidInput("recurrence", "task is not recurring")
			}
			t.SkipRecurrence()
			return nil
		})
}

// DeferRecurrence is a temporary compatibility action that pushes a recurring
// task's due date forward by a positive number of days.
func (s *Service) DeferRecurrence(taskID string, days int) (RecurrenceResult, error) {
	if days <= 0 {
		return RecurrenceResult{}, invalidInput("defer", fmt.Sprintf("days must be a positive number, got %d", days))
	}
	return s.mutateRecurrence(taskID, "Defer Recurrence",
		func(t *models.Task) string { return fmt.Sprintf("%d days #%d", days, t.ShortID) },
		func(t *models.Task) error {
			if t.Recurrence == nil {
				return invalidInput("recurrence", "task is not recurring")
			}
			t.DeferRecurrence(days)
			return nil
		})
}

func (s *Service) DeferRecurrenceByLocator(locator TaskLocator, days int) (RecurrenceResult, error) {
	if days <= 0 {
		return RecurrenceResult{}, invalidInput("defer", fmt.Sprintf("days must be a positive number, got %d", days))
	}
	return s.mutateRecurrenceByLocator(locator, "Defer Recurrence",
		func(t *models.Task) string { return fmt.Sprintf("%d days #%d", days, t.ShortID) },
		func(t *models.Task) error {
			if t.Recurrence == nil {
				return invalidInput("recurrence", "task is not recurring")
			}
			t.DeferRecurrence(days)
			return nil
		})
}
