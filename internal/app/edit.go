package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// EditRequest describes an in-place edit of a task's core content. Each pointer
// field is applied only when non-nil, so a caller changes just the fields it
// sets and leaves the rest untouched.
type EditRequest struct {
	// TaskID identifies the task to edit when it is workspace-unique.
	TaskID string
	// Locator is the project-qualified alternative to TaskID. The two fields are
	// mutually exclusive.
	Locator TaskLocator
	// Title, when non-nil, replaces the task title (validated).
	Title *string
	// Description, when non-nil, replaces the task description.
	Description *string
}

// Edit applies content changes to a task and commits them as one ChangeSet. A
// request that changes nothing is still valid and re-commits the project
// unchanged; validation of a non-nil title runs before any mutation.
func (s *Service) Edit(req EditRequest) (TaskResult, error) {
	ref, snap, revision, err := s.loadTaskSelection(req.TaskID, req.Locator)
	if err != nil {
		return TaskResult{}, err
	}
	before, err := cloneSnapshot(snap)
	if err != nil {
		return TaskResult{}, fmt.Errorf("capture edit rollback state: %w", err)
	}
	useRollbackDocuments(before)
	committed := false
	defer func() {
		if !committed {
			restoreSnapshotProjects(snap, before)
		}
	}()
	if applyErr := s.applyEdit(ref.Task, req); applyErr != nil {
		return TaskResult{}, applyErr
	}
	result, err := s.commitProject(ref, revision, "Edit Task", "")
	if err != nil {
		return TaskResult{}, err
	}
	committed = true
	return result, nil
}

// validateEditStateTransition makes a state-changing mutation obey the same
// guards as the canonical Transition use case. Content-only edits and explicit
// same-state writes remain valid; a real transition checks the state graph and
// incomplete children before mutation.
func validateEditStateTransition(ref TaskRef, target *models.WorkflowState) error {
	if target == nil {
		return nil
	}
	if !target.IsValid() {
		return invalidInput("workflow state", fmt.Sprintf("%q is not a valid workflow state", *target))
	}
	from := ref.Task.State()
	if *target == from {
		return nil
	}
	return validateLifecycleTransition(ref, *target)
}

// applyEdit is the shared validation and mutation seam for task edits.
func (s *Service) applyEdit(task *models.Task, req EditRequest) error {
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if err := models.ValidateTaskTitle(title); err != nil {
			return invalidInput("title", err.Error())
		}
		task.Title = title
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	task.UpdatedAt = s.clock.Now()
	return nil
}

// cloneCalendarDatePtr returns the local calendar date represented by t with
// its clock component removed.
func cloneCalendarDatePtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	year, month, day := t.Date()
	value := time.Date(year, month, day, 0, 0, 0, 0, t.Location())
	return &value
}
