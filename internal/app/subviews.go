package app

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// This file holds the decision and checklist use cases backing the task subview
// panels. They mirror the comment use cases (comments.go): every mutation
// resolves the task fresh, validates the index against the current item count so
// a stale adapter-side index cannot silently target the wrong item, and commits
// exactly one ChangeSet.

// DecisionResult is the presentation-neutral outcome of a decision mutation.
type DecisionResult struct {
	// Task is the standard single-task mutation result.
	Task TaskResult
	// Index is the affected decision's index (for add, the new decision's index).
	Index int
	// Count is the number of decisions on the task after the mutation.
	Count int
	// Resolved is the decision's resolved state after the mutation (meaningful
	// for ToggleDecisionResolved only).
	Resolved bool
}

// AddDecision appends a decision (with optional rationale and category) to a
// task and commits it as one ChangeSet. Empty or whitespace-only text is
// rejected before any mutation.
func (s *Service) AddDecision(taskID, text, rationale string, category models.DecisionCategory) (DecisionResult, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return DecisionResult{}, invalidInput("decision", "text is required")
	}
	var out DecisionResult
	res, err := s.mutateTask(taskID, "Add Decision", "", func(ref TaskRef) error {
		ref.Task.AddDecision(trimmed, strings.TrimSpace(rationale), category)
		ref.Task.UpdatedAt = s.clock.Now()
		out.Index = len(ref.Task.Decisions) - 1
		out.Count = len(ref.Task.Decisions)
		return nil
	})
	if err != nil {
		return DecisionResult{}, err
	}
	out.Task = res
	return out, nil
}

// EditDecision replaces the text and rationale of the decision at the given
// index, preserving its original timestamp. A nil category keeps the existing
// category (the external-editor flow edits only text and rationale); a non-nil
// category replaces it.
func (s *Service) EditDecision(taskID string, index int, text, rationale string, category *models.DecisionCategory) (DecisionResult, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return DecisionResult{}, invalidInput("decision", "text is required")
	}
	var out DecisionResult
	res, err := s.mutateTask(taskID, "Edit Decision", "", func(ref TaskRef) error {
		if err := validSubviewIndex("decision", index, len(ref.Task.Decisions)); err != nil {
			return err
		}
		ref.Task.Decisions[index].Text = trimmed
		ref.Task.Decisions[index].Rationale = strings.TrimSpace(rationale)
		if category != nil {
			ref.Task.Decisions[index].Category = *category
		}
		ref.Task.UpdatedAt = s.clock.Now()
		out.Index = index
		out.Count = len(ref.Task.Decisions)
		return nil
	})
	if err != nil {
		return DecisionResult{}, err
	}
	out.Task = res
	return out, nil
}

// RemoveDecision removes the decision at the given index.
func (s *Service) RemoveDecision(taskID string, index int) (DecisionResult, error) {
	var out DecisionResult
	res, err := s.mutateTask(taskID, "Remove Decision", "", func(ref TaskRef) error {
		if err := validSubviewIndex("decision", index, len(ref.Task.Decisions)); err != nil {
			return err
		}
		if !ref.Task.RemoveDecision(index) {
			return invalidInput("decision", fmt.Sprintf("failed to remove decision at index %d", index))
		}
		ref.Task.UpdatedAt = s.clock.Now()
		out.Index = index
		out.Count = len(ref.Task.Decisions)
		return nil
	})
	if err != nil {
		return DecisionResult{}, err
	}
	out.Task = res
	return out, nil
}

// ToggleDecisionResolved flips the resolved state of the decision at the given
// index and reports the post-toggle state.
func (s *Service) ToggleDecisionResolved(taskID string, index int) (DecisionResult, error) {
	var out DecisionResult
	res, err := s.mutateTask(taskID, "Resolve Decision", "", func(ref TaskRef) error {
		if err := validSubviewIndex("decision", index, len(ref.Task.Decisions)); err != nil {
			return err
		}
		if !ref.Task.ToggleDecisionResolved(index) {
			return invalidInput("decision", fmt.Sprintf("failed to toggle decision at index %d", index))
		}
		ref.Task.UpdatedAt = s.clock.Now()
		out.Index = index
		out.Count = len(ref.Task.Decisions)
		out.Resolved = ref.Task.Decisions[index].Resolved
		return nil
	})
	if err != nil {
		return DecisionResult{}, err
	}
	out.Task = res
	return out, nil
}

// ChecklistResult is the presentation-neutral outcome of a checklist mutation.
type ChecklistResult struct {
	// Task is the standard single-task mutation result.
	Task TaskResult
	// Index is the affected item's index after the mutation (for add, the new
	// item's index; for reorder, the destination index).
	Index int
	// Count is the number of checklist items on the task after the mutation.
	Count int
	// Done is the item's done state after the mutation (meaningful for
	// ToggleChecklistItem only).
	Done bool
}

// AddChecklistItem appends an item to a task's checklist. Empty or
// whitespace-only text is rejected before any mutation.
func (s *Service) AddChecklistItem(taskID, text string) (ChecklistResult, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ChecklistResult{}, invalidInput("checklist", "text is required")
	}
	var out ChecklistResult
	res, err := s.mutateTask(taskID, "Add Checklist Item", "", func(ref TaskRef) error {
		ref.Task.Checklist = append(ref.Task.Checklist, models.ChecklistItem{Text: trimmed})
		ref.Task.UpdatedAt = s.clock.Now()
		out.Index = len(ref.Task.Checklist) - 1
		out.Count = len(ref.Task.Checklist)
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
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ChecklistResult{}, invalidInput("checklist", "text is required")
	}
	var out ChecklistResult
	res, err := s.mutateTask(taskID, "Edit Checklist Item", "", func(ref TaskRef) error {
		if err := validSubviewIndex("checklist", index, len(ref.Task.Checklist)); err != nil {
			return err
		}
		ref.Task.Checklist[index].Text = trimmed
		ref.Task.UpdatedAt = s.clock.Now()
		out.Index = index
		out.Count = len(ref.Task.Checklist)
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
	var out ChecklistResult
	res, err := s.mutateTask(taskID, "Remove Checklist Item", "", func(ref TaskRef) error {
		if err := validSubviewIndex("checklist", index, len(ref.Task.Checklist)); err != nil {
			return err
		}
		ref.Task.Checklist = append(ref.Task.Checklist[:index], ref.Task.Checklist[index+1:]...)
		ref.Task.UpdatedAt = s.clock.Now()
		out.Index = index
		out.Count = len(ref.Task.Checklist)
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
	var out ChecklistResult
	res, err := s.mutateTask(taskID, "Toggle Checklist Item", "", func(ref TaskRef) error {
		if err := validSubviewIndex("checklist", index, len(ref.Task.Checklist)); err != nil {
			return err
		}
		ref.Task.Checklist[index].Done = !ref.Task.Checklist[index].Done
		ref.Task.UpdatedAt = s.clock.Now()
		out.Index = index
		out.Count = len(ref.Task.Checklist)
		out.Done = ref.Task.Checklist[index].Done
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
	var out ChecklistResult
	res, err := s.mutateTask(taskID, "Reorder Checklist", "", func(ref TaskRef) error {
		if err := validSubviewIndex("checklist", from, len(ref.Task.Checklist)); err != nil {
			return err
		}
		if err := validSubviewIndex("checklist", to, len(ref.Task.Checklist)); err != nil {
			return err
		}
		ref.Task.Checklist[from], ref.Task.Checklist[to] = ref.Task.Checklist[to], ref.Task.Checklist[from]
		ref.Task.UpdatedAt = s.clock.Now()
		out.Index = to
		out.Count = len(ref.Task.Checklist)
		return nil
	})
	if err != nil {
		return ChecklistResult{}, err
	}
	out.Task = res
	return out, nil
}

// validSubviewIndex rejects an out-of-range item index against the current item
// count, mirroring validCommentIndex for the other subview collections.
func validSubviewIndex(kind string, index, count int) error {
	if index < 0 || index >= count {
		return invalidInput(kind, fmt.Sprintf("%s index %d out of range (task has %d)", kind, index, count))
	}
	return nil
}
