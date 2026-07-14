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
	// FocusArtifacts applies set_active side effects to the planning domain and
	// renders its revision-checked artifact. It is required when a rule requests
	// any Focus Queue change so that planning and task writes share one commit.
	FocusArtifacts WorkflowFocusArtifactBuilder
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
	if len(req.TaskIDs) == 0 {
		return BulkResult{}, invalidInput("tasks", "at least one task id is required")
	}
	snap, err := s.repo.Snapshot()
	if err != nil {
		return BulkResult{}, err
	}
	refs, err := resolveDistinct(snap, req.TaskIDs)
	if err != nil {
		return BulkResult{}, err
	}

	resolver := NewWorkspaceResolver(snap)
	if req.SkipInvalid {
		valid := refs[:0]
		for _, r := range refs {
			if s.bulkTransitionReject(resolver, r, req.Target) == nil {
				valid = append(valid, r)
			}
		}
		refs = valid
		if len(refs) == 0 {
			return BulkResult{Tasks: []TaskResult{}}, nil
		}
	} else {
		for _, r := range refs {
			if rejectionErr := s.bulkTransitionReject(resolver, r, req.Target); rejectionErr != nil {
				return BulkResult{}, rejectionErr
			}
		}
	}

	// Capture pre-transition states per source project before mutating, so
	// event-based rule triggers ("status changed to done") observe the change.
	var prevStates map[string]map[string]*models.Task
	if len(req.Rules) > 0 {
		prevStates = make(map[string]map[string]*models.Task)
		for _, r := range refs {
			if _, ok := prevStates[r.Project.ID]; !ok {
				prevStates[r.Project.ID] = workflow.CaptureTaskStates(r.Project.Tasks)
			}
		}
	}

	for _, r := range refs {
		applyState(r.Task, req.Target)
		r.Task.UpdatedAt = s.clock.Now()
	}

	commits, idx := distinctProjectCommits(refs, snap)

	triggerCounts := make(map[string]int)
	var focusChanges []WorkflowFocusChange
	if len(req.Rules) > 0 {
		// Run the cascade per source project. Move destinations join the same
		// commit set; a destination that is itself a source is committed once.
		committed := make(map[string]bool, len(commits))
		for _, c := range commits {
			committed[c.project.ID] = true
		}
		sources := len(commits)
		for i := 0; i < sources; i++ {
			project := commits[i].project
			added, focus, cascadeErr := s.applyBulkWorkflowCascade(snap, project, req.Rules, prevStates[project.ID], committed, triggerCounts)
			if cascadeErr != nil {
				return BulkResult{}, cascadeErr
			}
			commits = append(commits, added...)
			focusChanges = append(focusChanges, focus...)
		}
	}
	resolvedFocus, err := attachWorkflowFocusArtifacts(commits, focusChanges, req.FocusArtifacts)
	if err != nil {
		return BulkResult{}, err
	}

	undoAction := req.UndoAction
	if undoAction == "" {
		undoAction = "Bulk Change State"
	}
	undoContext := req.UndoContext
	if undoContext == "" {
		undoContext = string(req.Target)
	}
	results, err := s.commitProjects(commits, undoAction, undoContext)
	if err != nil {
		return BulkResult{}, err
	}

	out := BulkResult{Tasks: make([]TaskResult, 0, len(refs))}
	if len(triggerCounts) > 0 {
		out.RuleTriggerCounts = triggerCounts
	}
	out.FocusChanges = resolvedFocus
	for _, r := range refs {
		out.Tasks = append(out.Tasks, newTaskResult(r, revisionAt(results, idx[r.Project.ID])))
	}
	for _, c := range commits {
		out.TouchedProjectIDs = append(out.TouchedProjectIDs, c.project.ID)
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

// applyBulkWorkflowCascade evaluates the task-level and project-level workflow
// rules against one already-mutated source project and applies everything the
// legacy interactive pipeline applied: follow-up tasks are appended with fresh
// short IDs, move_to_project actions relocate subtrees into their destination
// projects (returned as additional commits when not already part of the
// transaction), the project's status icon is updated by project-level rules
// unless the user pinned it, and every fired rule's trigger count accumulates
// into counts. Field conflicts leave the disputed field unchanged, matching the
// legacy pipeline, which surfaced no conflict feedback on bulk saves.
func (s *Service) applyBulkWorkflowCascade(
	snap Snapshot,
	project *models.Project,
	rules []workflow.WorkflowRule,
	prev map[string]*models.Task,
	committed map[string]bool,
	counts map[string]int,
) ([]projectCommit, []WorkflowFocusChange, error) {
	var result *workflow.EvaluationResult
	if prev != nil {
		evalCtx := workflow.BuildEvaluationContext(project.Tasks, prev, project.Name, project.ID)
		result = workflow.EvaluateRulesWithContext(evalCtx, rules)
	} else {
		result = workflow.EvaluateRules(project.Tasks, rules)
	}

	for _, followup := range result.NewFollowupTasks {
		followup.ShortID = project.GetNextShortID()
		project.Tasks = append(project.Tasks, followup)
	}
	for id, n := range result.RuleTriggerCounts {
		counts[id] += n
	}
	focusChanges := appendWorkflowFocusChanges(nil, project.ID, result.FocusChanges)

	var added []projectCommit
	for _, move := range result.DeferredMoves {
		task := project.GetTaskByID(move.TaskID)
		if task == nil {
			continue
		}
		dest := resolveWorkflowMoveDest(snap, move.Destination)
		if dest == nil || dest.ID == project.ID {
			continue
		}
		if _, err := project.MoveTaskToProject(task, dest); err != nil {
			return nil, nil, fmt.Errorf("workflow move_to_project failed: %w", err)
		}
		if !committed[dest.ID] {
			committed[dest.ID] = true
			added = append(added, projectCommit{project: dest, expectedRevision: snap.Revision(dest.ID), base: snap.Document(dest.ID)})
		}
	}

	// Project-level rules derive a status icon from aggregate task data; a
	// user-pinned icon is never overwritten.
	if project.StatusIconSource != "user" {
		projResult := workflow.EvaluateProjectRules(&workflow.ProjectEvaluationContext{
			Project: project,
			Tasks:   project.Tasks,
		}, rules)
		if projResult.NewStatusIcon != "" {
			project.StatusIcon = projResult.NewStatusIcon
			project.StatusIconSource = "auto"
			for _, id := range projResult.TriggeredRuleIDs {
				counts[id]++
			}
		}
	}
	return added, focusChanges, nil
}
