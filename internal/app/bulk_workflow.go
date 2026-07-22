package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

// BulkTransitionRequest asks to move several tasks — possibly spanning projects —
// to one target workflow state as a single transaction, optionally running the
// workflow-rule cascade the interactive bulk pipeline expects: event-based rules
// evaluated against the pre-mutation task states, created follow-up tasks,
// cross-project move_to_project actions, and project-level status-icon rules all
// commit atomically with the transitions themselves.
type BulkTransitionRequest struct {
	// TaskIDs identifies each task to transition. At least one is required;
	// duplicates are collapsed.
	TaskIDs []string
	// TaskLocators is the project-qualified alternative used by cross-project
	// clients. It is mutually exclusive with TaskIDs.
	TaskLocators []TaskLocator
	// Target is the workflow state every task moves to. Required.
	Target models.WorkflowState
	// Rules, when non-empty, enables the workflow cascade after the transitions
	// are applied. Rule evaluation sees the pre-transition states as the event
	// context, so "status changed to X" triggers fire exactly as they do on a
	// single-task save.
	Rules []workflow.WorkflowRule
	// UndoAction labels the ledger entry; empty defaults to "Bulk Change State".
	UndoAction string
	// UndoContext describes the batch; empty defaults to the target state.
	UndoContext string
	// ProjectStatusArtifacts applies project-rule status effects to a checked
	// workspace-catalog artifact in the same transaction.
	ProjectStatusArtifacts WorkflowProjectStatusArtifactBuilder
	// PlanningArtifacts folds the durable planning reconciliation (Today/Next
	// exclusivity) the batch implies into the same transaction.
	PlanningArtifacts PlanningArtifactBuilder
	// SkipInvalid switches the batch from all-or-nothing to best-effort: tasks
	// that cannot transition (invalid transition, dependency-blocked completion,
	// or an incomplete-children guard) are dropped from the batch instead of
	// aborting it. This matches the TUI multiselect pipeline, which operates on
	// the valid subset of the selection and reports the count actually changed.
	// With no valid task left the request commits nothing and returns an empty
	// result rather than an error.
	SkipInvalid bool
}

// BulkTransition moves several tasks to a target workflow state, committing every
// touched project as ONE transaction. Tasks may span projects. The batch validates
// every transition first — an invalid transition or a dependency-blocked completion
// aborts the whole batch before any commit — so the mutation is all-or-nothing.
func (s *Service) BulkTransition(taskIDs []string, target models.WorkflowState) (BulkResult, error) {
	return s.BulkTransitionWithWorkflow(BulkTransitionRequest{TaskIDs: taskIDs, Target: target})
}

// BulkTransitionWithWorkflow is BulkTransition plus the optional workflow-rule
// cascade. Every touched project — transition sources, follow-up carriers, and
// move_to_project destinations — commits together as one transaction, so the whole
// batch (and everything the rules cascaded from it) lands or nothing does. Rule
// trigger counts are returned for the adapter to persist to its config store; they
// are deliberately not part of the transaction because trigger counters live in
// workspace config, not project markdown, and were best-effort on the legacy path
// too. A task relocated by a move_to_project rule reports its result against its
// source project reference.
func (s *Service) BulkTransitionWithWorkflow(req BulkTransitionRequest) (BulkResult, error) {
	if !req.Target.IsValid() {
		return BulkResult{}, invalidInput("state", fmt.Sprintf("%q is not a valid workflow state", req.Target))
	}
	if len(req.TaskIDs) == 0 && len(req.TaskLocators) == 0 {
		return BulkResult{}, invalidInput("tasks", "at least one task id is required")
	}
	undoAction := req.UndoAction
	if undoAction == "" {
		undoAction = "Bulk Change State"
	}
	undoContext := req.UndoContext
	if undoContext == "" {
		undoContext = string(req.Target)
	}
	req.PlanningArtifacts = s.defaultedPlanningArtifacts(req.PlanningArtifacts)

	var refs []TaskRef
	var projectIndexes map[string]int
	orchestrated, err := s.runWorkflowOrchestration(workflowOrchestrationRequest{
		rules:                  req.Rules,
		projectStatusArtifacts: req.ProjectStatusArtifacts,
		planningArtifacts:      req.PlanningArtifacts,
		undoAction:             undoAction,
		undoContext:            undoContext,
		forceCommit:            true,
		prepare: func(snap Snapshot) (workflowMutationPlan, error) {
			resolved, err := resolveRequestedTasks(snap, req.TaskIDs, req.TaskLocators)
			if err != nil {
				return workflowMutationPlan{}, err
			}
			resolver := NewWorkspaceResolver(snap)
			if req.SkipInvalid {
				valid := resolved[:0]
				for _, ref := range resolved {
					if s.bulkTransitionReject(resolver, ref, req.Target) == nil {
						valid = append(valid, ref)
					}
				}
				resolved = valid
			} else {
				for _, ref := range resolved {
					if rejectionErr := s.bulkTransitionReject(resolver, ref, req.Target); rejectionErr != nil {
						return workflowMutationPlan{}, rejectionErr
					}
				}
			}
			refs = resolved
			if len(refs) == 0 {
				return workflowMutationPlan{skipCommit: true}, nil
			}

			previous := make(map[string]map[string]*models.Task)
			if len(req.Rules) > 0 {
				for _, ref := range refs {
					if _, ok := previous[ref.Project.ID]; !ok {
						previous[ref.Project.ID] = workflow.CaptureTaskStates(ref.Project.Tasks)
					}
				}
			}
			commits, indexes := distinctProjectCommits(refs, snap)
			projectIndexes = indexes
			sources := make([]*models.Project, len(commits))
			for i := range commits {
				sources[i] = commits[i].project
			}
			planningTaskLocators := make([]TaskLocator, 0, len(refs))
			for _, ref := range refs {
				planningTaskLocators = append(planningTaskLocators, TaskLocator{ProjectID: ref.Project.ID, TaskID: ref.Task.ID})
			}
			return workflowMutationPlan{
				commits:              commits,
				sources:              sources,
				previousStates:       previous,
				planningTaskLocators: planningTaskLocators,
				mutate: func(_ *WorkspaceResolver) error {
					for _, ref := range refs {
						applyState(ref.Task, req.Target)
						ref.Task.UpdatedAt = s.clock.Now()
					}
					return nil
				},
			}, nil
		},
	})
	if err != nil {
		return BulkResult{}, err
	}

	out := BulkResult{Tasks: make([]TaskResult, 0, len(refs))}
	out.RuleTriggerCounts = orchestrated.ruleTriggerCounts
	out.Conflicts = orchestrated.conflicts
	out.ProjectStatusChanges = orchestrated.projectStatusChanges
	for _, ref := range refs {
		out.Tasks = append(out.Tasks, newTaskResult(ref, revisionAt(orchestrated.commitOutcome.results, projectIndexes[ref.Project.ID])))
	}
	for _, commit := range orchestrated.commits {
		out.TouchedProjectIDs = append(out.TouchedProjectIDs, commit.project.ID)
	}
	return out, nil
}

// bulkTransitionReject reports why a task cannot move to target, or nil when the
// transition is allowed. It is the single predicate both the all-or-nothing and
// the best-effort (SkipInvalid) batch paths consult, so the two modes agree on
// exactly which transitions are valid.
func (s *Service) bulkTransitionReject(resolver *WorkspaceResolver, r TaskRef, target models.WorkflowState) error {
	from := r.Task.State()
	if reason := models.CanTransitionState(from, target); reason != "" {
		return &TransitionError{From: string(from), To: string(target), Reason: reason}
	}
	if target == models.StateDone && resolver.HasIncompleteDependencies(r.Task, r.Project) {
		return fmt.Errorf("%w: task #%d", ErrDependencyBlocked, r.Task.ShortID)
	}
	return childrenCompletionGuard(r.Task, target)
}
