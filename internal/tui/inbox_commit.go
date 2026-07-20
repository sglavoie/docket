package tui

import (
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workflow"
)

// inboxTriageUndoAction labels the workspace-ledger entry recorded for a single
// inbox triage mutation, so `tt undo --global` names the change meaningfully.
const inboxTriageUndoAction = "Inbox Triage"

// commitInboxProject commits an already-mutated inbox project through the
// app-layer transactional repository — the journaled commit service with the
// workspace undo ledger attached — then reloads it from disk so the caller's
// in-memory copy reflects the committed transaction. It replaces the retired
// raw saveProjectSync path: the inbox operations still mutate their in-memory
// project in place (and keep their own view-local undo history), but the durable
// write now flows through the same transactional boundary every other TUI
// mutation uses. Workflow evaluation and parent reconciliation run in the same
// transaction, so completing a final nested child can safely update ancestors.
func commitInboxProject(dataDir string, deps *config.RuntimeDependencies, ledger *storage.WorkspaceLedger, manager *config.Manager, runtime *config.RuntimeState, project *models.Project) error {
	if project == nil {
		return nil
	}
	var rules []workflow.WorkflowRule
	if runtime != nil {
		rules = runtime.Automation().GetEnabledWorkflowRules()
	}
	service := newTUIAppService(dataDir, deps, ledger)
	if _, err := service.CommitProjectWithWorkflow(project, app.ProjectWorkflowOptions{
		Rules:                  rules,
		UndoAction:             inboxTriageUndoAction,
		ProjectStatusArtifacts: workflowProjectStatusArtifactBuilder(manager),
		// Inbox state and section triage changes a task's derived workflow state, so
		// fold Today/Next exclusivity into the same transaction. Naming every task in
		// the committed project guarantees the triaged task is reconciled; a task
		// whose state did not change reconciles to no planning artifact.
		PlanningArtifacts: planningReconcileArtifactBuilder(manager),
		PlanningTaskIDs:   inboxProjectTaskIDs(project),
	}); err != nil {
		return err
	}
	return storage.LoadProject(project)
}

// inboxProjectTaskIDs returns every task ID in the inbox project so the planning
// reconciler enforces Today/Next exclusivity for the triaged task regardless of
// which task in the whole-project commit changed state.
func inboxProjectTaskIDs(project *models.Project) []string {
	ids := make([]string, 0, len(project.Tasks))
	for _, t := range project.Tasks {
		if t != nil {
			ids = append(ids, t.ID)
		}
	}
	return ids
}
