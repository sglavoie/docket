package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

// EditRequest describes an in-place edit of a task's core content. Each pointer
// field is applied only when non-nil, so a caller changes just the fields it
// sets and leaves the rest untouched.
type EditRequest struct {
	// Locator is the project-qualified task to edit.
	Locator TaskLocator
	// Title, when non-nil, replaces the task title (validated).
	Title *string
	// Description, when non-nil, replaces the task description.
	Description *string
	// CheckIn, when non-nil, replaces the task's whole check-in schedule; an
	// empty schedule clears it. Nil preserves what the task already holds, which
	// is what keeps every edit that never mentions a check-in — a title fix, a
	// body rewrite, an external-editor round trip — from silently retiring one.
	// Replacing is not acknowledging: the review flag and LastReviewedAt survive.
	CheckIn *models.CheckInSchedule
}

// Edit applies content changes to a task and commits them as one ChangeSet. A
// request that changes nothing is still valid and re-commits the project
// unchanged; validation of a non-nil title runs before any mutation.
func (s *Service) Edit(req EditRequest) (TaskResult, error) {
	ref, snap, revision, err := s.loadTaskSelection("", req.Locator)
	if err != nil {
		return TaskResult{}, err
	}
	before := cloneSnapshot(snap)
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
	now := s.clock.Now()
	if req.CheckIn != nil {
		if !req.CheckIn.IsEmpty() && task.IsTerminalState() {
			return invalidInput("check-in", fmt.Sprintf(
				"#%d is %s; a finished task owes no check-in", task.ShortID, task.State()))
		}
		if err := applyCheckInSchedule(task, *req.CheckIn, now); err != nil {
			return err
		}
	}
	task.UpdatedAt = now
	return nil
}

// cloneInstantPtr copies t whole. Deferral is the one date field that keeps its
// clock component, because a deferral can expire later the same day.
func cloneInstantPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	value := *t
	return &value
}

// cloneDueDatePtr returns a copy of the due-date pointer t, preserving its
// clock component. A date-only due date arrives here already normalized to
// local midnight; a timed due date keeps its hh:mm so models.Task.DueHasTime
// can derive from the clock. It replaces the earlier date-only clone now that a
// due date may carry an explicit time.
func cloneDueDatePtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	value := *t
	return &value
}
