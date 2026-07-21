package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// StateRequest asks to move a task to a target workflow state.
type StateRequest struct {
	// TaskID identifies the task to transition. Required.
	TaskID string
	// Target is the destination workflow state.
	Target models.WorkflowState
	// PlanningArtifacts, when set, folds the durable planning reconciliation the
	// transition implies (Today/Next exclusivity) into the same ledger
	// transaction as the task write, so undo restores task state and planning
	// membership together. Nil keeps the bare transition.
	PlanningArtifacts PlanningArtifactBuilder
}

// Transition moves a task to a target workflow state, enforcing the canonical
// transition rules (nonterminal states are freely interchangeable and may
// complete or cancel; terminal states may only reopen to inbox). Completing a
// task that still has unresolved dependencies is refused with
// ErrDependencyBlocked; the check resolves cross-project dependencies against the
// whole workspace, so a task blocked only by an already-completed task in another
// project can complete. On success one ChangeSet is committed and recurring
// instances are generated for a freshly completed recurring task.
func (s *Service) Transition(req StateRequest) (TaskResult, error) {
	target := req.Target
	if !target.IsValid() {
		return TaskResult{}, invalidInput("state", fmt.Sprintf("%q is not a valid workflow state", target))
	}
	ref, snap, revision, err := s.loadTask(req.TaskID)
	if err != nil {
		return TaskResult{}, err
	}
	from := ref.Task.State()
	if reason := models.CanTransitionState(from, target); reason != "" {
		return TaskResult{}, &TransitionError{From: string(from), To: string(target), Reason: reason}
	}
	if target == models.StateDone && NewWorkspaceResolver(snap).HasIncompleteDependencies(ref.Task, ref.Project) {
		return TaskResult{}, fmt.Errorf("%w: task #%d", ErrDependencyBlocked, ref.Task.ShortID)
	}
	if guardErr := childrenCompletionGuard(ref.Task, target); guardErr != nil {
		return TaskResult{}, guardErr
	}
	applyState(ref.Task, target)
	ref.Task.UpdatedAt = s.clock.Now()
	artifacts, err := s.transitionPlanningArtifacts(ref, s.defaultedPlanningArtifacts(req.PlanningArtifacts))
	if err != nil {
		return TaskResult{}, err
	}
	return s.commitProjectWithArtifacts(ref, revision, artifacts, "Change State", string(target))
}

// transitionPlanningArtifacts renders the planning reconciliation a single-task
// transition implies through the caller-supplied builder, returning the checked
// artifacts to fold into the transition's changeset. A nil builder or an empty
// build yields no artifacts, leaving the commit a bare task write.
func (s *Service) transitionPlanningArtifacts(ref TaskRef, builder PlanningArtifactBuilder) ([]storage.ArtifactChange, error) {
	if builder == nil {
		return nil, nil
	}
	targets := []PlanningReconcileTarget{{
		TaskID:    ref.Task.ID,
		ProjectID: ref.Project.ID,
		State:     ref.Task.State(),
	}}
	artifacts, err := builder(targets)
	if err != nil {
		return nil, err
	}
	for _, artifact := range artifacts {
		if artifact.ExpectedRevision == nil {
			return nil, fmt.Errorf("app: planning artifact %q has no expected revision", artifact.Path)
		}
	}
	return artifacts, nil
}

// Complete is the completion convenience over Transition: it moves a task to the
// terminal done state. It is refused when the task still has unresolved
// dependencies or is already terminal.
func (s *Service) Complete(taskID string) (TaskResult, error) {
	return s.Transition(StateRequest{TaskID: taskID, Target: models.StateDone})
}

// childrenCompletionGuard rejects a terminal transition (done or canceled) of a
// task that still has incomplete direct children, mirroring the legacy CLI
// completion invariant: a parent must never become done or canceled while a
// child is still open. It is a no-op for nonterminal targets and for leaf tasks,
// and it runs before any mutation so a rejected parent never enters the illegal
// completed-with-incomplete-child state that the post-mutation reconciliation
// would otherwise silently undo.
func childrenCompletionGuard(task *models.Task, target models.WorkflowState) error {
	if !target.IsTerminal() {
		return nil
	}
	incomplete := task.IncompleteChildren()
	if len(incomplete) == 0 {
		return nil
	}
	return newChildrenBlockedError(task, incomplete)
}

// newChildrenBlockedError builds a ChildrenBlockedError from the blocking child
// tasks, capturing the presentation-neutral detail (short ID, title, status)
// adapters render.
func newChildrenBlockedError(task *models.Task, children []*models.Task) error {
	kids := make([]IncompleteChild, 0, len(children))
	for _, c := range children {
		kids = append(kids, IncompleteChild{ShortID: c.ShortID, Title: c.Title, Status: string(c.State())})
	}
	return &ChildrenBlockedError{ParentShortID: task.ShortID, Children: kids}
}

// applyState routes a validated target state to the domain lifecycle method that
// realizes it, so state changes reuse the exact field mutations (completion
// timestamps, snooze clearing, section moves) the domain already guarantees.
func applyState(task *models.Task, target models.WorkflowState) {
	switch target {
	case models.StateDone:
		task.Complete()
	case models.StateCanceled:
		task.Cancel()
	case models.StateInbox:
		// Reopen returns a terminal task to inbox and also serves a nonterminal
		// move to inbox: it clears any completion timestamp and sets todo+inbox.
		task.Reopen()
	case models.StateInProgress:
		task.Start()
	case models.StateReview:
		task.Review()
	case models.StateWaiting:
		task.Block()
	case models.StatePaused:
		task.Standby()
	case models.StateSomeday:
		task.Maybe()
	case models.StateNext, models.StateReady:
		// Ready and Next carry no dedicated lifecycle method; the canonical
		// legacy-field mapping realizes them (todo, with/without the Next section).
		task.ApplyWorkflowState(target)
	}
}
