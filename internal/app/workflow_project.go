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
	// PrevStates (taskID -> pre-mutation snapshot) enables contextual, event-based
	// rule evaluation. A caller that captured the project's task states before
	// mutating supplies it; a nil map selects stateless evaluation, matching the
	// legacy persistWithWorkflow behavior (nil PrevStates -> EvaluateRules).
	PrevStates map[string]*models.Task
	// UndoAction / UndoContext label the ledger entry. UndoAction defaults to
	// "Task Modified" when empty, mirroring persistWithWorkflow's safety net.
	UndoAction  string
	UndoContext string
	// FocusArtifacts applies set_active side effects to a revision-checked
	// planning artifact committed with this project mutation.
	FocusArtifacts WorkflowFocusArtifactBuilder
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

	// The caller supplies an already-mutated project, but its persistence base is
	// still the exact on-disk document. Load one repository snapshot for every
	// path (including a bare mutation) so the commit is revision-checked and can
	// preserve opaque content instead of falling back to last-write-wins.
	snap, err := s.repo.Snapshot()
	if err != nil {
		return BulkResult{}, err
	}
	if snap.ProjectByID(project.ID) == nil {
		return BulkResult{}, fmt.Errorf("%w: %s", ErrProjectNotFound, project.ID)
	}

	triggerCounts := make(map[string]int)
	var focusChanges []WorkflowFocusChange
	commits := []projectCommit{{
		project:          project,
		expectedRevision: snap.Revision(project.ID),
		base:             snap.Document(project.ID),
	}}

	if len(opts.Rules) > 0 {
		committed := map[string]bool{project.ID: true}
		added, focus, cerr := s.applyBulkWorkflowCascade(snap, project, opts.Rules, opts.PrevStates, committed, triggerCounts)
		if cerr != nil {
			return BulkResult{}, cerr
		}
		commits = append(commits, added...)
		focusChanges = append(focusChanges, focus...)
	}
	resolvedFocus, err := attachWorkflowFocusArtifacts(commits, focusChanges, opts.FocusArtifacts)
	if err != nil {
		return BulkResult{}, err
	}

	if _, err := s.commitProjects(commits, undoAction, opts.UndoContext); err != nil {
		return BulkResult{}, err
	}

	out := BulkResult{}
	if len(triggerCounts) > 0 {
		out.RuleTriggerCounts = triggerCounts
	}
	out.FocusChanges = resolvedFocus
	for _, c := range commits {
		out.TouchedProjectIDs = append(out.TouchedProjectIDs, c.project.ID)
	}
	return out, nil
}
