package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
)

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

// applyPendingWeeklyReviewDelete credits the Weekly Review completion summary
// for a committed canonical deletion it originated. token is the committing
// batch's correlation token: the count is credited and cleared only by the exact
// result the pending count armed (the map entry keyed by that token). An
// unrelated delete success, a non-delete success, any mismatched result (zero or
// unarmed token), or a duplicate delivery of an already-consumed result is a
// no-op that LEAVES every other in-flight count intact, so each overlapping
// Weekly Review delete is credited exactly once and none is erased before its own
// delete returns.
func (m *AppModel) applyPendingWeeklyReviewDelete(token uint64, removedIDs []string) {
	if token == 0 {
		return
	}
	count, ok := m.pendingWeeklyReviewDeletes[token]
	if !ok {
		return
	}
	delete(m.pendingWeeklyReviewDeletes, token)
	if count <= 0 || len(removedIDs) == 0 || m.views == nil {
		return
	}
	if wr, ok := m.views.Get(ViewWeeklyReview).(*WeeklyReviewModel); ok && wr != nil {
		wr.recordCommittedDeletion(count)
	}
}

func (m *AppModel) clearPendingDeleteBookkeeping(token uint64) {
	if token == 0 {
		return
	}
	delete(m.pendingWeeklyReviewDeletes, token)
}

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
	origin := m.pendingTaskDelete.origin
	m.clearPendingTaskDelete()
	if len(roots) == 0 {
		return nil
	}

	// Mint a per-request correlation token. It rides the committing result/error
	// (stampDeleteRequestToken) so the Inbox and Weekly Review post-commit
	// bookkeeping armed below is consumed only by THIS delete's outcome, never by
	// an unrelated command that happens to finish first.
	m.deleteRequestSeq++
	token := m.deleteRequestSeq

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
	// revision-checked planning-domain artifact. Machine-local recent visits are
	// repaired separately by the shell after the commit (handleBatchTransactionResult).
	var artifacts []storage.ArtifactChange
	if m.runtime != nil && m.configManager != nil {
		planning := clonePlanningState(m.runtime.Planning())
		if planning.RemoveDurableTaskLocators(configLocators(locatorsToDelete)) > 0 {
			arts, err := planningDomainArtifact(m.configManager, planning)
			if err != nil {
				// This aborts before the bulk transaction command is created, so its
				// ErrorMsg never reaches the TransactionErrorMsg cleanup path. Drop both
				// the shell token slot armed above and the originating view's
				// per-task focus slot armed in requestDelete, keyed by THIS request's
				// token, so the failed delete leaves no stale in-flight bookkeeping to
				// suppress a later final-task close. The task stays in the queue.
				m.clearPendingDeleteBookkeeping(token)
				return errorCmd("%v", err)
			}
			artifacts = arts
		}
	}

	// Arm Weekly Review post-commit deletion accounting. This runs only after the
	// artifact build above succeeded, so an artifact-build error early-return never
	// arms it. The count is the number of requested roots (the confirmation-prompt
	// count) so the Deletions row uses the same per-task unit as the other
	// single-task summary counters; it is applied only on commit
	// (applyPendingWeeklyReviewDelete), never on cancellation or failure.
	if origin == ViewWeeklyReview {
		if m.pendingWeeklyReviewDeletes == nil {
			m.pendingWeeklyReviewDeletes = make(map[uint64]int)
		}
		m.pendingWeeklyReviewDeletes[token] = len(roots)
	}

	locators := taskLocatorsFromTaskRefs(roots)
	cmd := m.secondaryFeatures.Bulk.DeleteLocators(locators, artifacts...)
	return stampDeleteRequestToken(finalizeLocatedBatchResult(cmd, locators, ""), token)
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
