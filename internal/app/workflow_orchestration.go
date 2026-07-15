package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

// workflowOrchestrationRequest describes the adapter-independent stages around
// a workflow-aware mutation. prepare resolves and validates targets from the one
// fresh snapshot, captures any contextual previous states, and returns a mutate
// callback that the runner invokes before reconciliation and rule evaluation.
type workflowOrchestrationRequest struct {
	rules                  []workflow.WorkflowRule
	focusArtifacts         WorkflowFocusArtifactBuilder
	projectStatusArtifacts WorkflowProjectStatusArtifactBuilder
	undoAction             string
	undoContext            string
	forceCommit            bool
	prepare                func(Snapshot) (workflowMutationPlan, error)
}

type workflowMutationPlan struct {
	commits        []projectCommit
	sources        []*models.Project
	previousStates map[string]map[string]*models.Task
	mutate         func(*WorkspaceResolver) error
	skipCommit     bool
	undoContext    string
}

// workflowOrchestrationOutcome carries every presentation-neutral effect of the
// shared pipeline. Public workflow APIs project this common result into their
// established WorkflowResult or BulkResult shapes.
type workflowOrchestrationOutcome struct {
	commits              []projectCommit
	commitOutcome        groupedCommitOutcome
	committed            bool
	modifiedTaskIDs      []string
	followupTaskIDs      []string
	moves                []WorkflowMove
	conflicts            []WorkflowConflict
	focusChanges         []WorkflowFocusChange
	projectStatusChanges []WorkflowProjectStatusChange
	ruleTriggerCounts    map[string]int
	recurringByProjectID map[string]int
	changedByWorkflow    bool
}

// bulkResultFromWorkflow projects the common cascade effects and touched
// projects into the public BulkResult used by workflow-aware mutations.
func bulkResultFromWorkflow(out workflowOrchestrationOutcome) BulkResult {
	result := BulkResult{
		RuleTriggerCounts:    out.ruleTriggerCounts,
		FocusChanges:         out.focusChanges,
		Conflicts:            out.conflicts,
		ProjectStatusChanges: out.projectStatusChanges,
	}
	for _, commit := range out.commits {
		if commit.project != nil {
			result.TouchedProjectIDs = append(result.TouchedProjectIDs, commit.project.ID)
		}
	}
	return result
}

// taskResultFromWorkflow resolves a mutation's primary task after deferred
// moves, so its result names the final owning project and revision. Recurring
// instances remain attributed to the mutation's source project because they are
// generated during source-project reconciliation before a workflow move.
func taskResultFromWorkflow(out workflowOrchestrationOutcome, original TaskRef, recurringProjectID string) TaskResult {
	final := original
	revision := ""
	for i, commit := range out.commits {
		if commit.project == nil {
			continue
		}
		if task := commit.project.GetTaskByID(original.Task.ID); task != nil {
			final = TaskRef{Task: task, Project: commit.project, Base: commit.base}
			if out.committed {
				revision = revisionAt(out.commitOutcome.results, i)
			} else {
				revision = commit.expectedRevision
			}
			break
		}
	}
	result := newTaskResult(final, revision)
	result.RecurringGenerated = out.recurringByProjectID[recurringProjectID]
	return result
}

// runWorkflowOrchestration is the single application-layer pipeline used by
// every existing workflow-aware mutation. It owns snapshot loading, mutation,
// the legacy pre-rule reconciliation pass, task/project rule cascades, deferred
// effects, final post-mutation hooks, and the one grouped transaction.
func (s *Service) runWorkflowOrchestration(req workflowOrchestrationRequest) (workflowOrchestrationOutcome, error) {
	if req.prepare == nil {
		return workflowOrchestrationOutcome{}, fmt.Errorf("app: workflow orchestration requires a preparation stage")
	}

	snap, err := s.repo.Snapshot()
	if err != nil {
		return workflowOrchestrationOutcome{}, err
	}
	plan, err := req.prepare(snap)
	if err != nil {
		return workflowOrchestrationOutcome{}, err
	}
	if plan.mutate != nil {
		if mutateErr := plan.mutate(NewWorkspaceResolver(snap)); mutateErr != nil {
			return workflowOrchestrationOutcome{}, mutateErr
		}
	}

	out := workflowOrchestrationOutcome{
		commits:              plan.commits,
		recurringByProjectID: make(map[string]int, len(plan.commits)),
	}
	for _, project := range plan.sources {
		if project == nil {
			continue
		}
		out.recurringByProjectID[project.ID] += reconcileBeforeWorkflowEvaluation(project)
	}

	committed := make(map[string]bool, len(plan.commits))
	for _, commit := range plan.commits {
		if commit.project != nil {
			committed[commit.project.ID] = true
		}
	}
	if len(req.rules) > 0 {
		for _, project := range plan.sources {
			if project == nil {
				continue
			}
			previous := plan.previousStates[project.ID]
			if cascadeErr := s.applyWorkflowCascade(snap, project, req.rules, previous, committed, &out); cascadeErr != nil {
				return workflowOrchestrationOutcome{}, cascadeErr
			}
		}
	}

	resolvedFocus, err := attachWorkflowFocusArtifacts(out.commits, out.focusChanges, req.focusArtifacts)
	if err != nil {
		return workflowOrchestrationOutcome{}, err
	}
	out.focusChanges = resolvedFocus
	resolvedProjectStatus, err := attachWorkflowProjectStatusArtifacts(out.commits, out.projectStatusChanges, req.projectStatusArtifacts)
	if err != nil {
		return workflowOrchestrationOutcome{}, err
	}
	out.projectStatusChanges = resolvedProjectStatus
	if plan.skipCommit {
		return out, nil
	}

	// ApplyWorkflows keeps its established no-op behavior: conflict-only and
	// no-rule evaluations do not manufacture a revision. User mutations force a
	// commit even when no workflow rule fires.
	if !req.forceCommit && !out.changedByWorkflow {
		return out, nil
	}
	if len(out.commits) == 0 {
		return workflowOrchestrationOutcome{}, fmt.Errorf("app: workflow orchestration produced no project commits")
	}

	undoContext := req.undoContext
	if plan.undoContext != "" {
		undoContext = plan.undoContext
	}
	commitOutcome, err := s.commitProjects(out.commits, req.undoAction, undoContext)
	if err != nil {
		return workflowOrchestrationOutcome{}, err
	}
	out.commitOutcome = commitOutcome
	out.committed = true
	for projectID, count := range commitOutcome.recurringByProjectID {
		out.recurringByProjectID[projectID] += count
	}
	return out, nil
}

// reconcileBeforeWorkflowEvaluation restores the legacy ordering: state-derived
// reconciliation happens after the user mutation but before rules inspect the
// project. Final hooks run again inside commitProjects because rules themselves
// may complete tasks, unblock dependents, or create recurring instances.
func reconcileBeforeWorkflowEvaluation(project *models.Project) int {
	project.CheckAndAutoUnblock()
	project.ReconcileParentCompletion()
	return project.GenerateRecurringInstances()
}

// applyWorkflowCascade evaluates and applies task-level and project-level rules
// for one source project. Move destinations join the existing grouped commit,
// while counters remain data returned to adapters rather than durable artifacts.
func (s *Service) applyWorkflowCascade(
	snap Snapshot,
	project *models.Project,
	rules []workflow.WorkflowRule,
	previous map[string]*models.Task,
	committed map[string]bool,
	out *workflowOrchestrationOutcome,
) error {
	var evaluated *workflow.EvaluationResult
	if previous != nil {
		evalCtx := workflow.BuildEvaluationContext(project.Tasks, previous, project.Name, project.ID)
		evaluated = workflow.EvaluateRulesWithContext(evalCtx, rules)
	} else {
		evaluated = workflow.EvaluateRules(project.Tasks, rules)
	}

	for _, conflict := range evaluated.Conflicts {
		out.conflicts = append(out.conflicts, WorkflowConflict{
			TaskID:    conflict.TaskID,
			TaskTitle: conflict.TaskTitle,
			Field:     conflict.Field,
			Rules:     append([]string(nil), conflict.Rules...),
		})
	}
	for _, task := range evaluated.ModifiedTasks {
		if task == nil {
			continue
		}
		out.modifiedTaskIDs = append(out.modifiedTaskIDs, task.ID)
		out.changedByWorkflow = true
	}
	for _, followup := range evaluated.NewFollowupTasks {
		followup.ShortID = project.GetNextShortID()
		project.Tasks = append(project.Tasks, followup)
		out.followupTaskIDs = append(out.followupTaskIDs, followup.ID)
		out.changedByWorkflow = true
	}
	for ruleID, count := range evaluated.RuleTriggerCounts {
		if out.ruleTriggerCounts == nil {
			out.ruleTriggerCounts = make(map[string]int)
		}
		out.ruleTriggerCounts[ruleID] += count
	}
	out.focusChanges = appendWorkflowFocusChanges(out.focusChanges, project.ID, evaluated.FocusChanges)
	if len(evaluated.FocusChanges) > 0 {
		out.changedByWorkflow = true
	}

	for _, move := range evaluated.DeferredMoves {
		task := project.GetTaskByID(move.TaskID)
		if task == nil {
			continue
		}
		destination := resolveWorkflowMoveDest(snap, move.Destination)
		if destination == nil || destination.ID == project.ID {
			continue
		}
		if _, err := project.MoveTaskToProject(task, destination); err != nil {
			return fmt.Errorf("workflow move_to_project failed: %w", err)
		}
		if !committed[destination.ID] {
			committed[destination.ID] = true
			out.commits = append(out.commits, projectCommit{
				project:          destination,
				expectedRevision: snap.Revision(destination.ID),
				base:             snap.Document(destination.ID),
			})
		}
		out.moves = append(out.moves, WorkflowMove{TaskID: move.TaskID, DestProjectID: destination.ID})
		out.changedByWorkflow = true
	}

	if project.StatusIconSource != "user" {
		projectResult := workflow.EvaluateProjectRules(&workflow.ProjectEvaluationContext{
			Project: project,
			Tasks:   project.Tasks,
		}, rules)
		if projectResult.NewStatusIcon != "" {
			project.StatusIcon = projectResult.NewStatusIcon
			project.StatusIconSource = "auto"
			out.projectStatusChanges = append(out.projectStatusChanges, WorkflowProjectStatusChange{
				ProjectID:        project.ID,
				StatusIcon:       project.StatusIcon,
				StatusIconSource: project.StatusIconSource,
			})
			out.changedByWorkflow = true
			for _, ruleID := range projectResult.TriggeredRuleIDs {
				if out.ruleTriggerCounts == nil {
					out.ruleTriggerCounts = make(map[string]int)
				}
				out.ruleTriggerCounts[ruleID]++
			}
		}
	}
	return nil
}
