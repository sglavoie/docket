package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// This file holds the checklist use cases backing the task subview panel. They
// mirror the comment use cases (comments.go): every mutation
// resolves the task fresh, validates the index against the current item count so
// a stale adapter-side index cannot silently target the wrong item, and commits
// exactly one ChangeSet.

// ChecklistResult is the presentation-neutral outcome of a checklist mutation.
type ChecklistResult struct {
	// Task is the standard single-task mutation result.
	Task TaskResult
}

// AddChecklistItem appends an item to a task's checklist. Empty or
// whitespace-only text is rejected before any mutation.
func (s *Service) AddChecklistItem(taskID, text string) (ChecklistResult, error) {
	return s.addChecklistItemSelection(taskID, TaskLocator{}, text)
}

func (s *Service) AddChecklistItemByLocator(locator TaskLocator, text string) (ChecklistResult, error) {
	return s.addChecklistItemSelection("", locator, text)
}

func (s *Service) addChecklistItemSelection(taskID string, locator TaskLocator, text string) (ChecklistResult, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ChecklistResult{}, invalidInput("checklist", "text is required")
	}
	var out ChecklistResult
	res, err := s.mutateRichRecordSelection(taskID, locator, "Add Checklist Item", func(ref TaskRef) error {
		ref.Task.Checklist = append(ref.Task.Checklist, models.ChecklistItem{Text: trimmed})
		ref.Task.UpdatedAt = s.clock.Now()
		return nil
	})
	if err != nil {
		return ChecklistResult{}, err
	}
	out.Task = res
	return out, nil
}

// EditChecklistItem replaces the text of the item at the given index,
// preserving its done state.
func (s *Service) EditChecklistItem(taskID string, index int, text string) (ChecklistResult, error) {
	return s.editChecklistItemSelection(taskID, TaskLocator{}, index, text)
}

func (s *Service) EditChecklistItemByLocator(locator TaskLocator, index int, text string) (ChecklistResult, error) {
	return s.editChecklistItemSelection("", locator, index, text)
}

func (s *Service) editChecklistItemSelection(taskID string, locator TaskLocator, index int, text string) (ChecklistResult, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ChecklistResult{}, invalidInput("checklist", "text is required")
	}
	var out ChecklistResult
	res, err := s.mutateRichRecordSelection(taskID, locator, "Edit Checklist Item", func(ref TaskRef) error {
		if err := validChecklistIndex(index, len(ref.Task.Checklist)); err != nil {
			return err
		}
		ref.Task.Checklist[index].Text = trimmed
		ref.Task.UpdatedAt = s.clock.Now()
		return nil
	})
	if err != nil {
		return ChecklistResult{}, err
	}
	out.Task = res
	return out, nil
}

// RemoveChecklistItem removes the item at the given index.
func (s *Service) RemoveChecklistItem(taskID string, index int) (ChecklistResult, error) {
	return s.removeChecklistItemSelection(taskID, TaskLocator{}, index)
}

func (s *Service) RemoveChecklistItemByLocator(locator TaskLocator, index int) (ChecklistResult, error) {
	return s.removeChecklistItemSelection("", locator, index)
}

func (s *Service) removeChecklistItemSelection(taskID string, locator TaskLocator, index int) (ChecklistResult, error) {
	var out ChecklistResult
	res, err := s.mutateRichRecordSelection(taskID, locator, "Remove Checklist Item", func(ref TaskRef) error {
		if err := validChecklistIndex(index, len(ref.Task.Checklist)); err != nil {
			return err
		}
		ref.Task.Checklist = append(ref.Task.Checklist[:index], ref.Task.Checklist[index+1:]...)
		ref.Task.UpdatedAt = s.clock.Now()
		return nil
	})
	if err != nil {
		return ChecklistResult{}, err
	}
	out.Task = res
	return out, nil
}

// ToggleChecklistItem flips the done state of the item at the given index and
// reports the post-toggle state.
func (s *Service) ToggleChecklistItem(taskID string, index int) (ChecklistResult, error) {
	return s.toggleChecklistItemSelection(taskID, TaskLocator{}, index)
}

func (s *Service) ToggleChecklistItemByLocator(locator TaskLocator, index int) (ChecklistResult, error) {
	return s.toggleChecklistItemSelection("", locator, index)
}

func (s *Service) toggleChecklistItemSelection(taskID string, locator TaskLocator, index int) (ChecklistResult, error) {
	var out ChecklistResult
	res, err := s.mutateRichRecordSelection(taskID, locator, "Toggle Checklist Item", func(ref TaskRef) error {
		if err := validChecklistIndex(index, len(ref.Task.Checklist)); err != nil {
			return err
		}
		ref.Task.Checklist[index].Done = !ref.Task.Checklist[index].Done
		ref.Task.UpdatedAt = s.clock.Now()
		return nil
	})
	if err != nil {
		return ChecklistResult{}, err
	}
	out.Task = res
	return out, nil
}

// ReorderChecklistItem swaps the items at from and to, matching the panel's
// single-step move semantics.
func (s *Service) ReorderChecklistItem(taskID string, from, to int) (ChecklistResult, error) {
	return s.reorderChecklistItemSelection(taskID, TaskLocator{}, from, to)
}

func (s *Service) ReorderChecklistItemByLocator(locator TaskLocator, from, to int) (ChecklistResult, error) {
	return s.reorderChecklistItemSelection("", locator, from, to)
}

func (s *Service) reorderChecklistItemSelection(taskID string, locator TaskLocator, from, to int) (ChecklistResult, error) {
	var out ChecklistResult
	res, err := s.mutateRichRecordSelection(taskID, locator, "Reorder Checklist", func(ref TaskRef) error {
		if err := validChecklistIndex(from, len(ref.Task.Checklist)); err != nil {
			return err
		}
		if err := validChecklistIndex(to, len(ref.Task.Checklist)); err != nil {
			return err
		}
		ref.Task.Checklist[from], ref.Task.Checklist[to] = ref.Task.Checklist[to], ref.Task.Checklist[from]
		ref.Task.UpdatedAt = s.clock.Now()
		return nil
	})
	if err != nil {
		return ChecklistResult{}, err
	}
	out.Task = res
	return out, nil
}

// MoveChecklistItemByLocator lifts the item at from out of the checklist and
// reinserts it at to, which is what repeated adjacent swaps produce on a
// contiguous slice. ReorderChecklistItem swaps and so cannot express an edge
// move across more than one position.
func (s *Service) MoveChecklistItemByLocator(locator TaskLocator, from, to int) (ChecklistResult, error) {
	var out ChecklistResult
	res, err := s.mutateRichRecordSelection("", locator, "Move Checklist Item", func(ref TaskRef) error {
		if err := validChecklistIndex(from, len(ref.Task.Checklist)); err != nil {
			return err
		}
		if err := validChecklistIndex(to, len(ref.Task.Checklist)); err != nil {
			return err
		}
		item := ref.Task.Checklist[from]
		remaining := slices.Delete(slices.Clone(ref.Task.Checklist), from, from+1)
		ref.Task.Checklist = slices.Insert(remaining, to, item)
		ref.Task.UpdatedAt = s.clock.Now()
		return nil
	})
	if err != nil {
		return ChecklistResult{}, err
	}
	out.Task = res
	return out, nil
}

// validChecklistIndex rejects an out-of-range item index against the current item
// count, mirroring validCommentIndex for the other subview collections.
func validChecklistIndex(index, count int) error {
	if index < 0 || index >= count {
		return invalidInput("checklist", fmt.Sprintf("checklist index %d out of range (task has %d)", index, count))
	}
	return nil
}
