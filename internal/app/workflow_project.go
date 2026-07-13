package app

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

// ProjectWorkflowOptions carries the workflow-rule cascade configuration for a
// project-scoped mutation. It is the project-scoped analogue of
// SingleTaskWorkflowOptions: the ProjectTree and Outliner views perform
// structural and multi-field mutations (reorder, indent/outdent, reparent, bulk
// field edits, section moves, paste) on the whole in-memory project rather than
// one field of one task, so they commit the already-mutated project through this
// seam while still running the legacy workflow-rule cascade.
type ProjectWorkflowOptions struct {
	// Rules, when non-empty, enables the workflow cascade after the mutation.
	// Empty rules commit the bare mutation with no cascade.
	Rules []workflow.WorkflowRule
	// PrevStates (taskID -> pre-mutation snapshot) enables contextual, event-based
	// rule evaluation. A caller that captured the project's task states before
	// mutating supplies it; a nil map selects stateless evaluation, matching the
	// legacy persistWithWorkflow behavior (nil PrevStates -> EvaluateRules).
	PrevStates map[string]*models.Task
	// UndoAction / UndoContext label the ledger entry. UndoAction defaults to
	// "Task Modified" when empty, mirroring persistWithWorkflow's safety net.
	UndoAction  string
	UndoContext string
}

// CommitProjectWithWorkflow commits an already-mutated project through the
// transactional repository, running the same workflow-rule cascade the legacy
// persistWithWorkflow pipeline ran — event-based rule evaluation (contextual when
// PrevStates is supplied, stateless otherwise), follow-up creation, deferred
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

	triggerCounts := make(map[string]int)
	// The legacy TUI save committed with an empty expected revision
	// (last-write-wins); a bare mutation with no cascade keeps that behavior and
	// avoids loading a second snapshot on the hot reorder/bulk path.
	commits := []projectCommit{{project: project, expectedRevision: ""}}

	if len(opts.Rules) > 0 {
		snap, err := s.repo.Snapshot()
		if err != nil {
			return BulkResult{}, err
		}
		commits[0].expectedRevision = snap.Revision(project.ID)
		committed := map[string]bool{project.ID: true}
		added, cerr := s.applyBulkWorkflowCascade(snap, project, opts.Rules, opts.PrevStates, committed, triggerCounts)
		if cerr != nil {
			return BulkResult{}, cerr
		}
		commits = append(commits, added...)
	}

	if _, err := s.commitProjects(commits, undoAction, opts.UndoContext); err != nil {
		return BulkResult{}, err
	}

	out := BulkResult{}
	if len(triggerCounts) > 0 {
		out.RuleTriggerCounts = triggerCounts
	}
	for _, c := range commits {
		out.TouchedProjectIDs = append(out.TouchedProjectIDs, c.project.ID)
	}
	return out, nil
}
