package app

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

// ProjectWorkflowOptions carries the workflow-rule cascade configuration for a
// project-scoped mutation. It is the project-scoped analog of
// SingleTaskWorkflowOptions: the ProjectTree and Outliner views perform
// structural and multi-field mutations (reorder, indent/outdent, reparent, bulk
// field edits, section moves, paste) on the whole in-memory project rather than
// one field of one task, so they commit the already-mutated project through this
// seam while still running the legacy workflow-rule cascade.
type ProjectWorkflowOptions struct {
	// Rules, when non-empty, enables the workflow cascade after the mutation.
	// Empty rules commit the bare mutation with no cascade.
	Rules []workflow.WorkflowRule
	// PrevStates may be supplied by a caller that captured the project before
	// mutating it. When nil, the service derives the previous states from the
	// repository snapshot's immutable base document.
	PrevStates map[string]*models.Task
	// UndoAction / UndoContext label the ledger entry. UndoAction defaults to
	// "Task Modified" when empty, mirroring persistWithWorkflow's safety net.
	UndoAction  string
	UndoContext string
	// FocusArtifacts applies set_active side effects to a revision-checked
	// planning artifact committed with this project mutation.
	FocusArtifacts WorkflowFocusArtifactBuilder
	// ProjectStatusArtifacts applies project-rule status effects to a checked
	// workspace-catalog artifact committed with this project mutation.
	ProjectStatusArtifacts WorkflowProjectStatusArtifactBuilder
}

// CommitProjectWithWorkflow commits an already-mutated project through the
// transactional repository, running contextual event-based rule evaluation,
// follow-up creation, deferred
// cross-project moves, project status-icon rules, and rule-trigger counters — and
// recording one grouped ledger entry across every touched project. The caller's
// project pointer (mutated in memory by the view) is the source of truth for the
// task write; the post-mutation hooks (auto-unblock, parent reconciliation,
// recurring generation, tree rebuild, completion reorder) run at commit time via
// commitProjects, exactly as persistWithWorkflow ran them.
//
// It returns a BulkResult carrying every touched project ID and any fired
// rule-trigger counts, so the TUI adapter reuses the batch result plumbing (reload
// every touched project, persist trigger counts to config) the bulk-with-workflow
// and single-task-with-workflow paths already use.
func (s *Service) CommitProjectWithWorkflow(project *models.Project, opts ProjectWorkflowOptions) (BulkResult, error) {
	if project == nil {
		return BulkResult{}, invalidInput("project", "project is required")
	}

	undoAction := opts.UndoAction
	if undoAction == "" {
		undoAction = "Task Modified"
	}

	orchestrated, err := s.runWorkflowOrchestration(workflowOrchestrationRequest{
		rules:                  opts.Rules,
		focusArtifacts:         opts.FocusArtifacts,
		projectStatusArtifacts: opts.ProjectStatusArtifacts,
		undoAction:             undoAction,
		undoContext:            opts.UndoContext,
		forceCommit:            true,
		prepare: func(snap Snapshot) (workflowMutationPlan, error) {
			if snap.ProjectByID(project.ID) == nil {
				return workflowMutationPlan{}, fmt.Errorf("%w: %s", ErrProjectNotFound, project.ID)
			}
			plan := workflowMutationPlan{
				commits: []projectCommit{{
					project:          project,
					expectedRevision: snap.Revision(project.ID),
					base:             snap.Document(project.ID),
				}},
				sources: []*models.Project{project},
			}
			if len(opts.Rules) > 0 {
				previous := opts.PrevStates
				if previous == nil {
					if base := snap.Document(project.ID); base != nil {
						previous = workflow.CaptureTaskStates(base.Tasks)
					} else {
						previous = map[string]*models.Task{}
					}
				}
				plan.previousStates = map[string]map[string]*models.Task{project.ID: previous}
			}
			return plan, nil
		},
	})
	if err != nil {
		return BulkResult{}, err
	}

	out := BulkResult{
		RuleTriggerCounts:    orchestrated.ruleTriggerCounts,
		FocusChanges:         orchestrated.focusChanges,
		Conflicts:            orchestrated.conflicts,
		ProjectStatusChanges: orchestrated.projectStatusChanges,
	}
	for _, commit := range orchestrated.commits {
		out.TouchedProjectIDs = append(out.TouchedProjectIDs, commit.project.ID)
	}
	return out, nil
}
