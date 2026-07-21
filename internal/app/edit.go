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
	// TaskID identifies the task to edit. Required.
	TaskID string
	// Title, when non-nil, replaces the task title (validated).
	Title *string
	// Description, when non-nil, replaces the task description.
	Description *string
	// Type, when non-nil, changes the task type and clears stale type-specific
	// attributes from the previous type.
	Type *models.TaskType
	// Form, when non-nil, replaces the task's complete editable field set from the
	// task form. Unlike the granular pointer fields above (used by `tt task edit`),
	// the task form always writes every editable field, so full replacement — not
	// per-field optionality — is the faithful semantics. Form and the granular
	// fields are mutually exclusive in practice.
	Form *FormEdit
	// WorkflowState, when non-nil, preserves or changes the canonical lifecycle
	// state after applying either a form or granular edit. Full-form callers need
	// this because the legacy section projection cannot distinguish Ready from In
	// Progress: both occupy the default active bucket.
	WorkflowState *models.WorkflowState
	// PlanningArtifacts, when set, folds the durable planning reconciliation the
	// edit implies (Today/Next exclusivity) into the same ledger transaction as the
	// task write, so a form edit that changes the task's section — and therefore its
	// workflow state — keeps Today/Next exclusive and undoes atomically. Nil uses
	// the service default planning builder (installed by the TUI); when neither is
	// set the edit is a bare task write.
	PlanningArtifacts PlanningArtifactBuilder
}

// FormEdit is the task form's full editable field set. Every field is applied
// unconditionally, mirroring the form's legacy in-place mutation: the draft holds
// the complete post-edit state, so the use case replaces rather than patches.
type FormEdit struct {
	Title         string
	Type          models.TaskType
	Priority      int
	ParentID      *string
	Section       string
	Description   string
	Tags          []string
	DueDate       *time.Time
	HasTime       bool
	StartDate     *time.Time
	EstimatedDays float64
	URL           string
	Source        string
	Dimmed        bool
	Pinned        bool
	Recurrence    *models.RecurrencePattern
	Checklist     []models.ChecklistItem
	// Type-specific attribute pointers. The form writes all five (only the active
	// type's pointer is non-nil), so assigning them replaces and clears stale
	// attributes from a previous type in one step.
	BugAttrs   *models.BugAttrs
	IdeaAttrs  *models.IdeaAttrs
	TaskAttrs  *models.TaskAttrs
	StoryAttrs *models.StoryAttrs
	EpicAttrs  *models.EpicAttrs
}

// Edit applies content changes to a task and commits them as one ChangeSet. A
// request that changes nothing is still valid and re-commits the project
// unchanged; validation of a non-nil title runs before any mutation. When
// req.Form is set, the full task-form field set is applied instead of the
// granular pointer fields.
func (s *Service) Edit(req EditRequest) (TaskResult, error) {
	ref, revision, err := s.resolveTask(req.TaskID)
	if err != nil {
		return TaskResult{}, err
	}
	if applyErr := s.applyEdit(ref.Task, req); applyErr != nil {
		return TaskResult{}, applyErr
	}
	// A form edit can change the task's section (and therefore its workflow state),
	// so fold the same Today/Next exclusivity reconciliation a state transition does
	// into the same transaction: entering StateNext drops Today, leaving StateNext
	// drops the stale Next order entry. A granular edit that leaves the state
	// unchanged reconciles to no planning change (nil artifacts), so this is a bare
	// task write for `tt task edit`.
	artifacts, err := s.transitionPlanningArtifacts(ref, s.defaultedPlanningArtifacts(req.PlanningArtifacts))
	if err != nil {
		return TaskResult{}, err
	}
	return s.commitProjectWithArtifacts(ref, revision, artifacts, "Edit Task", "")
}

// EditWithWorkflow applies the same edit mutation as Edit, then evaluates rules
// contextually against the true pre-edit task states captured from the shared
// workflow snapshot. Contextual evaluation is mandatory because edit rules may
// depend on field transitions.
func (s *Service) EditWithWorkflow(req EditRequest, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	if opts.UndoAction == "" {
		opts.UndoAction = "Edit Task"
	}
	// A form edit changes the derived workflow state when its section changes, so
	// fold the Today/Next exclusivity reconciliation into the same cascade
	// transaction. The single-task workflow runner already names the edited task as
	// its planning target, so the builder reconciles exactly that task.
	opts.PlanningArtifacts = s.defaultedPlanningArtifacts(opts.PlanningArtifacts)
	return s.mutateWithWorkflow(req.TaskID, opts, func(ref TaskRef, _ *WorkspaceResolver) error {
		return s.applyEdit(ref.Task, req)
	})
}

// applyEdit is the shared validation and mutation seam for bare and
// workflow-aware edits.
func (s *Service) applyEdit(task *models.Task, req EditRequest) error {
	originalState := task.State()
	if req.WorkflowState != nil {
		if !req.WorkflowState.IsValid() {
			return invalidInput("workflow state", fmt.Sprintf("%q is not a valid workflow state", *req.WorkflowState))
		}
		if *req.WorkflowState != originalState {
			if reason := models.CanTransitionState(originalState, *req.WorkflowState); reason != "" {
				return invalidInput("workflow state", reason)
			}
		}
	}
	if req.Form != nil {
		if err := s.applyFormEdit(task, *req.Form); err != nil {
			return err
		}
	} else {
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
		if req.Type != nil {
			if !req.Type.IsValid() {
				return invalidInput("type", string(*req.Type)+" is not a valid task type")
			}
			if task.Type != *req.Type {
				task.ClearTypeAttrs()
				task.Type = *req.Type
			}
		}
	}
	if req.WorkflowState != nil {
		if *req.WorkflowState == originalState {
			task.ApplyWorkflowState(originalState)
		} else {
			task.ApplyStateTransition(*req.WorkflowState)
		}
	}
	task.UpdatedAt = s.clock.Now()
	return nil
}

// applyFormEdit replaces a task's editable fields from the task form's draft,
// validating title, type, priority, section, and tags before mutating so a
// rejected edit never persists. It mirrors the field-application order the form
// used before the migration (title/type/priority/parent/section, then dates,
// tags, flags, recurrence, links, description, checklist, type attrs, pinned).
func (s *Service) applyFormEdit(task *models.Task, f FormEdit) error {
	title := strings.TrimSpace(f.Title)
	if err := models.ValidateTaskTitle(title); err != nil {
		return invalidInput("title", err.Error())
	}
	if !f.Type.IsValid() {
		return invalidInput("type", fmt.Sprintf("%q is not a valid task type", f.Type))
	}
	if err := models.ValidatePriority(f.Priority, false); err != nil {
		return invalidInput("priority", err.Error())
	}
	if f.Section != "" && !models.IsGTDSection(f.Section) {
		return invalidInput("section", fmt.Sprintf("%q is not a built-in GTD section; use tags for flexible grouping", f.Section))
	}
	normalizedTags, err := normalizeTags(f.Tags)
	if err != nil {
		return err
	}

	task.Title = title
	task.Type = f.Type
	task.Priority = f.Priority
	task.ParentID = f.ParentID
	task.SetSection(f.Section)
	task.DueDate = cloneTimePtr(f.DueDate)
	task.HasTime = f.HasTime
	task.StartDate = cloneTimePtr(f.StartDate)
	task.EstimatedDays = f.EstimatedDays
	task.Tags = append([]string{}, normalizedTags...)
	task.Dimmed = f.Dimmed
	task.Recurrence = f.Recurrence
	task.URL = f.URL
	task.Source = f.Source
	task.Description = strings.TrimSpace(f.Description)
	task.Checklist = filterChecklist(f.Checklist)
	task.BugAttrs = f.BugAttrs
	task.IdeaAttrs = f.IdeaAttrs
	task.TaskAttrs = f.TaskAttrs
	task.StoryAttrs = f.StoryAttrs
	task.EpicAttrs = f.EpicAttrs

	// Update PinnedAt only when the pinned flag actually changes, matching the
	// form's legacy behavior.
	if f.Pinned != task.Pinned {
		task.Pinned = f.Pinned
		if f.Pinned {
			now := s.clock.Now()
			task.PinnedAt = &now
		} else {
			task.PinnedAt = nil
		}
	}
	task.UpdatedAt = s.clock.Now()
	return nil
}

// cloneTimePtr returns a deep copy of a *time.Time, or nil for nil input, so a
// committed task never aliases the caller's time pointer.
func cloneTimePtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	cp := *t
	return &cp
}

// filterChecklist copies a checklist skipping blank-text items, matching the
// form and markdown parser, which both drop empty entries.
func filterChecklist(items []models.ChecklistItem) []models.ChecklistItem {
	var out []models.ChecklistItem
	for _, item := range items {
		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}
		out = append(out, models.ChecklistItem{Text: text, Done: item.Done})
	}
	return out
}
