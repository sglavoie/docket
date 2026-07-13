package app

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

// SingleTaskWorkflowOptions carries the workflow-rule cascade configuration
// shared by every workflow-aware single-task mutation. It is the single-task
// analogue of BulkTransitionRequest's rule fields.
//
// The secondary TUI views (all-tasks, weekly/daily review, priority, due-date,
// focus, focus-board, project-focus) historically ran the full workflow pipeline
// after a single-task save: event-based rule evaluation, follow-up creation,
// deferred cross-project moves, project status-icon rules, and rule-trigger
// counters. This options struct plus mutateWithWorkflow preserves that behavior
// while committing through the transactional repository and workspace ledger.
type SingleTaskWorkflowOptions struct {
	// Rules, when non-empty, enables the workflow cascade after the mutation.
	// Empty rules commit the bare mutation with no cascade.
	Rules []workflow.WorkflowRule
	// Contextual selects contextual rule evaluation (pre-mutation task states
	// captured as the event context) over stateless evaluation. The legacy
	// secondary-view single-task save path evaluated statelessly (it supplied no
	// prior state), so its callers leave this false; a caller that supplies a
	// prior-state snapshot (the primary views) sets it true.
	Contextual bool
	// UndoAction / UndoContext label the ledger entry. UndoAction defaults to
	// "Task Modified" when empty.
	UndoAction  string
	UndoContext string
}

// mutateWithWorkflow is the shared runner for a workflow-aware single-task
// mutation. It resolves the task, optionally captures the pre-mutation task
// states for contextual rule evaluation, applies mutate (which enforces domain
// guards and may return a typed error), runs the same workflow-rule cascade the
// bulk path runs — follow-up creation, deferred cross-project moves, project
// status-icon rules, and rule-trigger counters — and commits every touched
// project (the source plus any move destinations) as ONE transaction. A mutate
// error aborts before any commit, so a rejected mutation never persists.
//
// It returns a BulkResult carrying exactly one task result, every touched
// project ID, and any fired rule-trigger counts, so a TUI adapter reuses the same
// batch result plumbing (reload every touched project, persist trigger counts to
// config) the bulk-with-workflow path already uses.
func (s *Service) mutateWithWorkflow(taskID string, opts SingleTaskWorkflowOptions, mutate func(TaskRef, *WorkspaceResolver) error) (BulkResult, error) {
	ref, snap, revision, err := s.loadTask(taskID)
	if err != nil {
		return BulkResult{}, err
	}
	resolver := NewWorkspaceResolver(snap)

	var prev map[string]*models.Task
	if opts.Contextual && len(opts.Rules) > 0 {
		prev = workflow.CaptureTaskStates(ref.Project.Tasks)
	}

	if err := mutate(ref, resolver); err != nil {
		return BulkResult{}, err
	}
	ref.Task.UpdatedAt = s.clock.Now()

	commits := []projectCommit{{project: ref.Project, expectedRevision: revision}}
	triggerCounts := make(map[string]int)
	if len(opts.Rules) > 0 {
		committed := map[string]bool{ref.Project.ID: true}
		added, cerr := s.applyBulkWorkflowCascade(snap, ref.Project, opts.Rules, prev, committed, triggerCounts)
		if cerr != nil {
			return BulkResult{}, cerr
		}
		commits = append(commits, added...)
	}

	undoAction := opts.UndoAction
	if undoAction == "" {
		undoAction = "Task Modified"
	}
	results, err := s.commitProjects(commits, undoAction, opts.UndoContext)
	if err != nil {
		return BulkResult{}, err
	}

	out := BulkResult{Tasks: []TaskResult{newTaskResult(ref, revisionAt(results, 0))}}
	if len(triggerCounts) > 0 {
		out.RuleTriggerCounts = triggerCounts
	}
	for _, c := range commits {
		out.TouchedProjectIDs = append(out.TouchedProjectIDs, c.project.ID)
	}
	return out, nil
}

// StateWithWorkflow moves one task to a target workflow state and runs the
// workflow cascade, committing every touched project atomically. It enforces the
// same transition guards as Transition (canonical transition rules, dependency
// blocking on completion, and the incomplete-children guard) before mutating, so
// a rejected transition never persists.
func (s *Service) StateWithWorkflow(taskID string, target models.WorkflowState, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	if !target.IsValid() {
		return BulkResult{}, invalidInput("state", fmt.Sprintf("%q is not a valid workflow state", target))
	}
	return s.mutateWithWorkflow(taskID, opts, func(ref TaskRef, resolver *WorkspaceResolver) error {
		from := ref.Task.State()
		if reason := models.CanTransitionState(from, target); reason != "" {
			return &TransitionError{From: string(from), To: string(target), Reason: reason}
		}
		if target == models.StateDone && resolver.HasIncompleteDependencies(ref.Task, ref.Project) {
			return fmt.Errorf("%w: task #%d", ErrDependencyBlocked, ref.Task.ShortID)
		}
		if err := childrenCompletionGuard(ref.Task, target); err != nil {
			return err
		}
		applyState(ref.Task, target)
		return nil
	})
}

// SetPriorityWithWorkflow sets one task's priority and runs the workflow cascade.
func (s *Service) SetPriorityWithWorkflow(taskID string, priority int, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.mutateWithWorkflow(taskID, opts, func(ref TaskRef, _ *WorkspaceResolver) error {
		if err := models.ValidatePriority(priority, false); err != nil {
			return invalidInput("priority", err.Error())
		}
		ref.Task.Priority = priority
		return nil
	})
}

// SetDueDateWithWorkflow sets or clears (nil due) one task's due date and runs the
// workflow cascade. A nil due clears both the date and its clock component; the
// legacy secondary-view quick due-date path applied no state guard, so none is
// enforced here.
func (s *Service) SetDueDateWithWorkflow(taskID string, due *time.Time, hasTime bool, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.mutateWithWorkflow(taskID, opts, func(ref TaskRef, _ *WorkspaceResolver) error {
		if due == nil {
			ref.Task.DueDate = nil
			ref.Task.HasTime = false
			return nil
		}
		d := *due
		ref.Task.DueDate = &d
		ref.Task.HasTime = hasTime
		return nil
	})
}

// SnoozeWithWorkflow sets or clears (nil until) one task's snooze date and runs the
// workflow cascade. The legacy secondary-view quick-snooze path set the field with
// no terminal/in-progress guard, so this preserves that gate-free behavior (unlike
// the CLI-facing Snooze use case).
func (s *Service) SnoozeWithWorkflow(taskID string, until *time.Time, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.mutateWithWorkflow(taskID, opts, func(ref TaskRef, _ *WorkspaceResolver) error {
		if until == nil {
			ref.Task.SnoozedUntil = nil
			return nil
		}
		u := *until
		ref.Task.SnoozedUntil = &u
		return nil
	})
}

// SetSectionWithWorkflow moves one task to the workflow state a GTD section name
// maps to (Task.SetSection performs the move) and runs the workflow cascade. A
// non-empty section is ensured on the project, matching the legacy
// applySectionChange behavior.
func (s *Service) SetSectionWithWorkflow(taskID, section string, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.mutateWithWorkflow(taskID, opts, func(ref TaskRef, _ *WorkspaceResolver) error {
		ref.Task.SetSection(section)
		if section != "" {
			ref.Project.EnsureSection(section)
		}
		return nil
	})
}

// SetPinnedWithWorkflow sets one task's Pinned flag and runs the workflow
// cascade. A nil pinned toggles the flag (the standard-keymap "toggle pin" op);
// a non-nil pinned forces it. It preserves the legacy applyTogglePinned behavior
// (Pin/Unpin adjust the pinned ordering) while committing through the ledger.
func (s *Service) SetPinnedWithWorkflow(taskID string, pinned *bool, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.mutateWithWorkflow(taskID, opts, func(ref TaskRef, _ *WorkspaceResolver) error {
		switch {
		case pinned == nil:
			ref.Task.TogglePinned()
		case *pinned:
			ref.Task.Pin()
		default:
			ref.Task.Unpin()
		}
		return nil
	})
}

// ToggleDimmedWithWorkflow flips one task's Dimmed flag (clearing Active when
// dimming, per the model) and runs the workflow cascade.
func (s *Service) ToggleDimmedWithWorkflow(taskID string, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.mutateWithWorkflow(taskID, opts, func(ref TaskRef, _ *WorkspaceResolver) error {
		ref.Task.ToggleDimmed()
		return nil
	})
}
