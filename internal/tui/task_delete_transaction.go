package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
)

const operationTaskDelete OperationID = "tui.task.delete"

// executeTaskDeleteOperation turns the frozen action-sheet targets into the one
// canonical delete request. The registry never mutates task data: confirmation
// and the atomic transaction stay in handleTaskDeleteRequested/confirmTaskDelete,
// which is why the binding delegates confirmation instead of adding a second,
// generic prompt in front of the canonical one.
func (m *AppModel) executeTaskDeleteOperation(
	invocation OperationInvocation,
) OperationHandlerResult {
	refs := make([]TaskProjectRef, 0, len(invocation.Targets.Targets))
	for _, target := range invocation.Targets.Targets {
		task, project := m.taskAndProjectForIdentity(target.Task)
		if task == nil || project == nil {
			return OperationHandlerResult{Err: fmt.Errorf("task is no longer available")}
		}
		refs = append(refs, TaskProjectRef{Task: task, Project: project})
	}
	if len(refs) == 0 {
		return OperationHandlerResult{
			Err: fmt.Errorf("operation %s requires at least one task", invocation.Descriptor.ID),
		}
	}
	return OperationHandlerResult{Message: TaskDeleteRequestedMsg{
		Refs:   refs,
		Origin: invocation.Context.View,
	}}
}

// TaskDeleteRequestedMsg is the canonical, shell-owned request to permanently
// delete one or more task subtrees. Any surface (Outliner, Planner, Task
// Detail, review, …) emits it instead of running its own delete flow: the shell
// canonicalizes the roots, prompts for confirmation, and — only after the user
// confirms — executes ONE atomic BulkCoordinator.Delete transaction that folds
// the durable planning-ref cleanup into the same commit. Refs carry each root
// task with its owning project so heterogeneous cross-project selections delete
// together; Origin records the presentation that asked so post-commit behavior
// (cursor repair, closing Task Detail) can stay origin-aware.
type TaskDeleteRequestedMsg struct {
	Refs   []TaskProjectRef
	Origin PresentationID
}

// taskDeletePending holds the canonicalized roots and origin for an in-flight
// shell-owned task deletion awaiting confirmation.
type taskDeletePending struct {
	roots  []TaskProjectRef
	origin PresentationID
}

const taskDeleteConfirmAction = "task-delete"

// handleTaskDeleteRequested canonicalizes the requested roots, resolves the
// confirmation prompt, and arms the shell-owned confirm dialog. It never mutates
// task data — the delete transaction runs only after the user confirms in
// confirmTaskDelete. An empty or fully-invalid request is a no-op.
func (m *AppModel) handleTaskDeleteRequested(msg TaskDeleteRequestedMsg) tea.Cmd {
	roots := canonicalizeTaskDeleteRoots(msg.Refs)
	if len(roots) == 0 {
		return nil
	}
	m.pendingTaskDelete = taskDeletePending{roots: roots, origin: msg.Origin}
	m.taskDeleteConfirm.Show(taskDeleteConfirmAction, taskDeletePrompt(roots))
	return nil
}

// canonicalizeTaskDeleteRoots drops refs with a nil task/project or blank ID and
// removes duplicate roots by project-qualified identity, preserving first-seen
// order. Descendant resolution and nested-root collapsing happen later (in
// confirmTaskDelete and the application service); this only shapes the
// confirmation summary.
func canonicalizeTaskDeleteRoots(refs []TaskProjectRef) []TaskProjectRef {
	seen := make(map[TaskIdentity]bool, len(refs))
	roots := make([]TaskProjectRef, 0, len(refs))
	for _, ref := range refs {
		if ref.Task == nil || ref.Project == nil || ref.Task.ID == "" {
			continue
		}
		identity := ref.Identity()
		if seen[identity] {
			continue
		}
		seen[identity] = true
		roots = append(roots, ref)
	}
	return roots
}

// taskDeletePrompt renders "Delete task #N?" for a single root or
// "Delete N tasks?" for several, matching the canonical deletion grammar.
func taskDeletePrompt(roots []TaskProjectRef) string {
	if len(roots) == 1 {
		return fmt.Sprintf("Delete task #%d?", roots[0].Task.ShortID)
	}
	return fmt.Sprintf("Delete %d tasks?", len(roots))
}

// handleTaskDeleteConfirmKey gives the shell-owned task-delete confirmation the
// same modal key priority as project archive: while it is active it consumes
// every key (confirm on y/Y, cancel on n/N/Esc, swallow the rest) so nothing
// leaks into the underlying view. It returns handled=false only when no delete
// confirmation is pending.
func (m *AppModel) handleTaskDeleteConfirmKey(key string) (bool, tea.Cmd) {
	if !m.taskDeleteConfirm.IsActive() {
		return false, nil
	}
	confirmed, handled := m.taskDeleteConfirm.HandleKey(key)
	if !handled {
		return false, nil
	}
	if confirmed {
		return true, m.confirmTaskDelete()
	}
	switch key {
	case "n", "N", "esc":
		m.clearPendingTaskDelete()
	}
	return true, nil
}

// confirmTaskDelete executes the canonical deletion transaction for the pending
// roots. It clones the planning state, removes the durable references for the
// entire descendant set, builds the revision-checked planning artifact, and runs
// ONE BulkCoordinator.Delete for both single- and multi-root deletion. The
// returned command is decorated so the committed batch clears the requested
// selection IDs after the commit (finalizeBatchResult passes a TransactionError
// through untouched, leaving state intact on failure). An artifact-build error
// aborts before any mutation and preserves the pending selection's task data.
func (m *AppModel) confirmTaskDelete() tea.Cmd {
	roots := m.pendingTaskDelete.roots
	m.clearPendingTaskDelete()
	if len(roots) == 0 {
		return nil
	}

	// Expand every root to its whole subtree so the durable planning cleanup keys
	// on each removed task, not just the roots.
	locatorsToDelete := make(map[app.TaskLocator]bool)
	for _, group := range groupTaskProjectRefsByProject(roots) {
		rootIDs := make(map[string]bool, len(group.Items))
		for _, item := range group.Items {
			rootIDs[item.Task.ID] = true
		}
		for id := range collectDescendantIDs(group.Project.Tasks, rootIDs) {
			locatorsToDelete[app.TaskLocator{ProjectID: group.Project.ID, TaskID: id}] = true
		}
	}

	// Fold the durable planning-ref cleanup into the delete transaction as the
	// revision-checked planning-domain artifact.
	var artifacts []storage.ArtifactChange
	if m.runtime != nil && m.configManager != nil {
		planning := clonePlanningState(m.runtime.Planning())
		if planning.RemoveDurableTaskLocators(configLocators(locatorsToDelete)) > 0 {
			arts, err := planningDomainArtifact(m.configManager, planning)
			if err != nil {
				return errorCmd("%v", err)
			}
			artifacts = arts
		}
	}

	locators := taskLocatorsFromTaskRefs(roots)
	cmd := m.secondaryFeatures.Bulk.DeleteLocators(locators, artifacts...)
	return finalizeLocatedBatchResult(cmd, locators, "")
}

func configLocators(locators map[app.TaskLocator]bool) []config.ProjectTaskLocator {
	result := make([]config.ProjectTaskLocator, 0, len(locators))
	for locator := range locators {
		result = append(result, config.ProjectTaskLocator{ProjectID: locator.ProjectID, TaskID: locator.TaskID})
	}
	return result
}

// clearPendingTaskDelete discards the pending roots and dismisses the confirm
// dialog. It runs on cancellation and after the transaction is dispatched so the
// shell never re-prompts for a stale selection.
func (m *AppModel) clearPendingTaskDelete() {
	m.pendingTaskDelete = taskDeletePending{}
	m.taskDeleteConfirm.Cancel()
}
