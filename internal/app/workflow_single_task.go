package app

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

// SingleTaskWorkflowOptions carries the workflow-rule cascade configuration
// shared by every workflow-aware single-task mutation. It is the single-task
// analog of BulkTransitionRequest's rule fields.
//
// Every workflow-aware mutation captures its pre-mutation repository state so
// event and hierarchy rules have consistent semantics across callers.
type SingleTaskWorkflowOptions struct {
	// Rules, when non-empty, enables the workflow cascade after the mutation.
	// Empty rules commit the bare mutation with no cascade.
	Rules []workflow.WorkflowRule
	// UndoAction / UndoContext label the ledger entry. UndoAction defaults to
	// "Task Modified" when empty.
	UndoAction  string
	UndoContext string
	// ProjectStatusArtifacts applies project-rule status effects to a checked
	// workspace-catalog artifact committed with the project changes.
	ProjectStatusArtifacts WorkflowProjectStatusArtifactBuilder
	// PlanningArtifacts folds durable Today/Next reconciliation for the mutation's
	// final post-cascade state into the same ledger transaction. Nil uses the
	// service default when configured.
	PlanningArtifacts PlanningArtifactBuilder
}

// mutateWithWorkflow is the shared runner for a workflow-aware single-task
// mutation. It resolves the task, captures the pre-mutation task
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
	return s.mutateWithWorkflowSelection(taskID, TaskLocator{}, opts, nil, mutate)
}

func (s *Service) mutateWithWorkflowByLocator(locator TaskLocator, opts SingleTaskWorkflowOptions, mutate func(TaskRef, *WorkspaceResolver) error) (BulkResult, error) {
	return s.mutateWithWorkflowSelection("", locator, opts, nil, mutate)
}

// mutateWithWorkflowGuarded is mutateWithWorkflow plus a fresh-snapshot no-op
// predicate. Legacy CLI commands used read-only prechecks for operations such as
// clearing an already-empty collection. Rechecking those predicates here keeps a
// stale-revision retry from evaluating rules or creating a ledger entry after a
// concurrent writer has already made the requested mutation unnecessary.
func (s *Service) mutateWithWorkflowGuarded(
	taskID string,
	opts SingleTaskWorkflowOptions,
	skip func(TaskRef) bool,
	mutate func(TaskRef, *WorkspaceResolver) error,
) (BulkResult, error) {
	return s.mutateWithWorkflowSelection(taskID, TaskLocator{}, opts, skip, mutate)
}

func (s *Service) mutateWithWorkflowGuardedByLocator(
	locator TaskLocator,
	opts SingleTaskWorkflowOptions,
	skip func(TaskRef) bool,
	mutate func(TaskRef, *WorkspaceResolver) error,
) (BulkResult, error) {
	return s.mutateWithWorkflowSelection("", locator, opts, skip, mutate)
}

func (s *Service) mutateWithWorkflowSelection(
	taskID string,
	locator TaskLocator,
	opts SingleTaskWorkflowOptions,
	skip func(TaskRef) bool,
	mutate func(TaskRef, *WorkspaceResolver) error,
) (BulkResult, error) {
	undoAction := opts.UndoAction
	if undoAction == "" {
		undoAction = "Task Modified"
	}

	var ref TaskRef
	orchestrated, err := s.runWorkflowOrchestration(workflowOrchestrationRequest{
		rules:                  opts.Rules,
		projectStatusArtifacts: opts.ProjectStatusArtifacts,
		planningArtifacts:      opts.PlanningArtifacts,
		undoAction:             undoAction,
		undoContext:            opts.UndoContext,
		forceCommit:            true,
		prepare: func(snap Snapshot) (workflowMutationPlan, error) {
			resolved, err := resolveTaskSelection(snap, taskID, locator)
			if err != nil {
				return workflowMutationPlan{}, err
			}
			ref = resolved
			if skip != nil && skip(ref) {
				return workflowMutationPlan{skipCommit: true}, nil
			}
			plan := workflowMutationPlan{
				commits: []projectCommit{{
					project:          ref.Project,
					expectedRevision: snap.Revision(ref.Project.ID),
					base:             ref.Base,
				}},
				sources:         []*models.Project{ref.Project},
				planningTaskIDs: []string{ref.Task.ID},
				mutate: func(resolver *WorkspaceResolver) error {
					if err := mutate(ref, resolver); err != nil {
						return err
					}
					ref.Task.UpdatedAt = s.clock.Now()
					return nil
				},
			}
			if len(opts.Rules) > 0 {
				plan.previousStates = map[string]map[string]*models.Task{
					ref.Project.ID: workflow.CaptureTaskStates(ref.Project.Tasks),
				}
				plan.forcedChanges = map[string][]string{ref.Project.ID: {ref.Task.ID}}
			}
			return plan, nil
		},
	})
	if err != nil {
		return BulkResult{}, err
	}

	out := bulkResultFromWorkflow(orchestrated)
	out.Tasks = []TaskResult{taskResultFromWorkflow(orchestrated, ref, ref.Project.ID)}
	return out, nil
}

func withDefaultWorkflowUndoAction(opts SingleTaskWorkflowOptions, action string) SingleTaskWorkflowOptions {
	if opts.UndoAction == "" {
		opts.UndoAction = action
	}
	return opts
}

// StateWithWorkflow moves one task to a target workflow state and runs the
// workflow cascade, committing every touched project atomically. It enforces the
// same transition guards as Transition (canonical transition rules, dependency
// blocking on completion, and the incomplete-children guard) before mutating, so
// a rejected transition never persists.
func (s *Service) StateWithWorkflow(taskID string, target models.WorkflowState, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.stateWithWorkflow(taskID, TaskLocator{}, target, opts)
}

func (s *Service) StateWithWorkflowByLocator(locator TaskLocator, target models.WorkflowState, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.stateWithWorkflow("", locator, target, opts)
}

func (s *Service) stateWithWorkflow(taskID string, locator TaskLocator, target models.WorkflowState, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	if !target.IsValid() {
		return BulkResult{}, invalidInput("state", fmt.Sprintf("%q is not a valid workflow state", target))
	}
	// Retain direct-state defaulting for callers that depend on this public seam;
	// runWorkflowOrchestration also defaults as the backstop for rule cascades
	// initiated by mutations that do not directly change workflow state.
	opts.PlanningArtifacts = s.defaultedPlanningArtifacts(opts.PlanningArtifacts)
	return s.mutateWithWorkflowSelection(taskID, locator, opts, nil, func(ref TaskRef, resolver *WorkspaceResolver) error {
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
		return applyState(ref.Task, target, s.clock.Now())
	})
}

// SetPriorityWithWorkflow sets one task's priority and runs the workflow cascade.
func (s *Service) SetPriorityWithWorkflow(taskID string, priority int, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.setPriorityWithWorkflow(taskID, TaskLocator{}, priority, opts)
}

func (s *Service) SetPriorityWithWorkflowByLocator(locator TaskLocator, priority int, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.setPriorityWithWorkflow("", locator, priority, opts)
}

func (s *Service) setPriorityWithWorkflow(taskID string, locator TaskLocator, priority int, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.mutateWithWorkflowSelection(taskID, locator, opts, nil, func(ref TaskRef, _ *WorkspaceResolver) error {
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
	return s.setDueDateWithWorkflow(taskID, TaskLocator{}, due, hasTime, opts)
}

func (s *Service) SetDueDateWithWorkflowByLocator(locator TaskLocator, due *time.Time, hasTime bool, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.setDueDateWithWorkflow("", locator, due, hasTime, opts)
}

func (s *Service) setDueDateWithWorkflow(taskID string, locator TaskLocator, due *time.Time, hasTime bool, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.mutateWithWorkflowSelection(taskID, locator, opts, nil, func(ref TaskRef, _ *WorkspaceResolver) error {
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
	return s.snoozeWithWorkflow(taskID, TaskLocator{}, until, opts)
}

func (s *Service) SnoozeWithWorkflowByLocator(locator TaskLocator, until *time.Time, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.snoozeWithWorkflow("", locator, until, opts)
}

func (s *Service) snoozeWithWorkflow(taskID string, locator TaskLocator, until *time.Time, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.mutateWithWorkflowSelection(taskID, locator, opts, nil, func(ref TaskRef, _ *WorkspaceResolver) error {
		return s.applySnooze(ref.Task, until, false)
	})
}

// GuardedSnoozeWithWorkflow is the CLI-facing snooze variant. Unlike the
// intentionally gate-free secondary-view SnoozeWithWorkflow method, it rejects
// terminal and in-progress targets through the same mutation helper as Snooze.
// Clearing an already-clear snooze is a true no-op: no rules are evaluated and
// no transaction or ledger entry is created.
func (s *Service) GuardedSnoozeWithWorkflow(taskID string, until *time.Time, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.guardedSnoozeWithWorkflow(taskID, TaskLocator{}, until, opts)
}

func (s *Service) GuardedSnoozeWithWorkflowByLocator(locator TaskLocator, until *time.Time, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.guardedSnoozeWithWorkflow("", locator, until, opts)
}

func (s *Service) guardedSnoozeWithWorkflow(taskID string, locator TaskLocator, until *time.Time, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	opts = withDefaultWorkflowUndoAction(opts, "Snooze Task")
	return s.mutateWithWorkflowSelection(taskID, locator, opts, func(ref TaskRef) bool {
		return until == nil && ref.Task.SnoozedUntil == nil
	}, func(ref TaskRef, _ *WorkspaceResolver) error {
		return s.applySnooze(ref.Task, until, true)
	})
}

// SetPinnedWithWorkflow sets one task's Pinned flag and runs the workflow
// cascade. A nil pinned toggles the flag (the standard-keymap "toggle pin" op);
// a non-nil pinned forces it. It preserves the legacy applyTogglePinned behavior
// (Pin/Unpin adjust the pinned ordering) while committing through the ledger.
func (s *Service) SetPinnedWithWorkflow(taskID string, pinned *bool, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.setPinnedWithWorkflow(taskID, TaskLocator{}, pinned, opts)
}

func (s *Service) SetPinnedWithWorkflowByLocator(locator TaskLocator, pinned *bool, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.setPinnedWithWorkflow("", locator, pinned, opts)
}

func (s *Service) setPinnedWithWorkflow(taskID string, locator TaskLocator, pinned *bool, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.mutateWithWorkflowSelection(taskID, locator, opts, nil, func(ref TaskRef, _ *WorkspaceResolver) error {
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
	return s.toggleDimmedWithWorkflow(taskID, TaskLocator{}, opts)
}

func (s *Service) ToggleDimmedWithWorkflowByLocator(locator TaskLocator, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.toggleDimmedWithWorkflow("", locator, opts)
}

func (s *Service) toggleDimmedWithWorkflow(taskID string, locator TaskLocator, opts SingleTaskWorkflowOptions) (BulkResult, error) {
	return s.mutateWithWorkflowSelection(taskID, locator, opts, nil, func(ref TaskRef, _ *WorkspaceResolver) error {
		return toggleDimmedTask(ref.Task)
	})
}
