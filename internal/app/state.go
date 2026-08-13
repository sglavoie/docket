package app

import (
	"fmt"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// StateRequest asks to move a task to a target workflow state.
type StateRequest struct {
	// TaskID identifies the task to transition when it is workspace-unique.
	TaskID string
	// Locator is the project-qualified alternative to TaskID. The two fields are
	// mutually exclusive.
	Locator TaskLocator
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
// complete or cancel; terminal states may only reopen to inbox). On success one
// ChangeSet is committed. Completion is a
// terminal transition of the selected task and never creates another task.
func (s *Service) Transition(req StateRequest) (TaskResult, error) {
	target := req.Target
	if !target.IsValid() {
		return TaskResult{}, invalidInput("state", fmt.Sprintf("%q is not a valid workflow state", target))
	}
	ref, snap, revision, err := s.loadTaskSelection(req.TaskID, req.Locator)
	if err != nil {
		return TaskResult{}, err
	}
	before, err := cloneSnapshot(snap)
	if err != nil {
		return TaskResult{}, fmt.Errorf("capture transition rollback state: %w", err)
	}
	keepMutation := false
	defer func() {
		if !keepMutation {
			restoreSnapshotProjects(snap, before)
		}
	}()
	if transitionErr := s.applyLifecycleTransition(ref, target); transitionErr != nil {
		return TaskResult{}, transitionErr
	}
	artifacts, err := s.transitionPlanningArtifacts(ref, s.defaultedPlanningArtifacts(req.PlanningArtifacts), true)
	if err != nil {
		return TaskResult{}, err
	}
	result, err := s.commitProjectWithArtifacts(ref, revision, artifacts, "Change State", models.WorkflowStateGroupForState(target).Label())
	if err != nil {
		return TaskResult{}, err
	}
	keepMutation = true
	return result, nil
}

// transitionPlanningArtifacts renders the planning reconciliation a single-task
// transition implies through the caller-supplied builder, returning the checked
// artifacts to fold into the transition's changeset. A nil builder or an empty
// build yields no artifacts, leaving the commit a bare task write.
func (s *Service) transitionPlanningArtifacts(ref TaskRef, builder PlanningArtifactBuilder, stateChanged bool) ([]storage.ArtifactChange, error) {
	if builder == nil {
		return nil, nil
	}
	targets := []PlanningReconcileTarget{{
		TaskID:       ref.Task.ID,
		ProjectID:    ref.Project.ID,
		State:        ref.Task.State(),
		StateChanged: stateChanged,
	}}
	artifacts, err := builder(PlanningReconcileRequest{Targets: targets})
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
// terminal done state. It is refused when the task is already terminal.
func (s *Service) Complete(taskID string) (TaskResult, error) {
	return s.Transition(StateRequest{TaskID: taskID, Target: models.StateDone})
}

// CompleteByLocator completes one explicitly project-qualified task.
func (s *Service) CompleteByLocator(locator TaskLocator) (TaskResult, error) {
	return s.Transition(StateRequest{Locator: locator, Target: models.StateDone})
}

// validateLifecycleTransition is the one application-layer lifecycle policy.
// Every single-task, bulk, edit, action-plan, and planning entry
// point uses it before mutating state, so forbidden graph edges and direct-child
// blocks return the same typed errors everywhere.
func validateLifecycleTransition(ref TaskRef, target models.WorkflowState) error {
	return validateLifecycleTransitionFrom(ref.Task.State(), ref, target)
}

// validateLifecycleTransitionFrom validates a transition whose source state was
// captured before the task was mutated. Project-scoped UI commits use this form
// because they submit an already-edited project together with its immutable base.
func validateLifecycleTransitionFrom(from models.WorkflowState, ref TaskRef, target models.WorkflowState) error {
	if err := validateLifecycleStateEdge(from, target); err != nil {
		return err
	}
	return childrenCompletionGuard(ref.Task, ref.Project, target)
}

// validateLifecycleStateEdge validates only the state-graph half of the policy.
// An entry point that builds hierarchy as part of the same request uses it while
// parsing and runs childrenCompletionGuard separately once the final direct
// children are known, so a terminal destination cannot be judged against a
// hierarchy the request itself is about to replace.
func validateLifecycleStateEdge(from, target models.WorkflowState) error {
	if reason := models.CanTransitionState(from, target); reason != "" {
		return &TransitionError{From: string(from), To: string(target), Reason: reason}
	}
	return nil
}

// applyLifecycleTransition validates and applies one transition with the service
// clock. Model state, terminal/reopen timestamps, snooze cleanup, and UpdatedAt
// therefore change through the same primitive for every application entry point.
func (s *Service) applyLifecycleTransition(ref TaskRef, target models.WorkflowState) error {
	if err := validateLifecycleTransition(ref, target); err != nil {
		return err
	}
	if err := applyState(ref.Task, target, s.clock.Now()); err != nil {
		return invalidInput("state", err.Error())
	}
	return nil
}

// childrenCompletionGuard rejects a terminal transition (done or canceled) of a
// task that still has incomplete direct children, mirroring the legacy CLI
// completion invariant: a parent must never become done or canceled while a
// child is still open. It is a no-op for nonterminal targets and for leaf tasks,
// and it runs before any mutation so a rejected parent never enters the illegal
// completed-with-incomplete-child state that the post-mutation reconciliation
// would otherwise silently undo.
func childrenCompletionGuard(task *models.Task, project *models.Project, target models.WorkflowState) error {
	if !target.IsTerminal() {
		return nil
	}
	incomplete := incompleteDirectChildren(task, project)
	if len(incomplete) == 0 {
		return nil
	}
	return newChildrenBlockedError(task, incomplete)
}

// incompleteDirectChildren derives the guard from durable ParentID links when
// the owning project is available. This remains correct even when a project-wide
// edit changed hierarchy before rebuilding the in-memory Children pointers.
func incompleteDirectChildren(task *models.Task, project *models.Project) []*models.Task {
	if task == nil {
		return nil
	}
	if project == nil {
		return task.IncompleteChildren()
	}
	var incomplete []*models.Task
	for _, candidate := range project.Tasks {
		if candidate == nil || candidate.ParentID == nil || *candidate.ParentID != task.ID || candidate.IsTerminalState() {
			continue
		}
		incomplete = append(incomplete, candidate)
	}
	return incomplete
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

// applyState is the application-layer state mutation seam. It supplies the
// service clock to the canonical model primitive so state, completion, snooze,
// and update timestamps change together deterministically. Callers validate
// transition policy before reaching this helper.
func applyState(task *models.Task, target models.WorkflowState, now time.Time) error {
	return task.ApplyStateTransitionAt(target, now)
}
