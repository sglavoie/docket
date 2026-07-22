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

// pendingInboxDelete records the inbox processor and task awaiting post-commit
// bookkeeping after a canonical deletion request. Origin distinguishes the
// per-project (ViewInboxProcessor) and global (ViewGlobalInboxProcessor)
// processors; identity is the root the shell was asked to remove; token is the
// per-request correlation token (deleteRequestSeq) that also keys this value in
// AppModel.pendingInboxDeletes, so the committing result must carry it before
// this bookkeeping is consumed.
type pendingInboxDelete struct {
	token    uint64
	origin   PresentationID
	identity TaskIdentity
}

// isInboxOrigin reports whether a delete request came from one of the inbox
// processors, which need explicit post-commit queue bookkeeping.
func isInboxOrigin(origin PresentationID) bool {
	return origin == ViewInboxProcessor || origin == ViewGlobalInboxProcessor
}

// applyPendingInboxDelete forwards a committed canonical deletion to the inbox
// processor that requested it so the processor advances, marks the root
// processed, and re-anchors its cursor. token is the committing batch's
// correlation token: the bookkeeping is consumed only by the exact result the
// pending state armed (the map entry keyed by that token). An unrelated batch
// (zero or unarmed token) is a no-op that LEAVES every other in-flight delete
// armed, so a command — or a second same-surface delete — that finishes first
// can never strand a committed Inbox delete.
func (m *AppModel) applyPendingInboxDelete(token uint64, removedLocators []app.TaskLocator, legacyRemovedIDs []string) tea.Cmd {
	if token == 0 {
		return nil
	}
	pending, ok := m.pendingInboxDeletes[token]
	if !ok || !pending.identity.Valid() {
		return nil
	}
	delete(m.pendingInboxDeletes, token)
	if len(removedLocators) > 0 {
		removed := make(map[TaskIdentity]bool, len(removedLocators))
		for _, locator := range removedLocators {
			removed[taskIdentityFromLocator(locator)] = true
		}
		if !removed[pending.identity] {
			return nil
		}
	} else if !taskIDSliceToSet(legacyRemovedIDs)[pending.identity.TaskID] {
		// Retain ID-only correlation solely for legacy tests/results that predate
		// RemovedTaskLocators. Production delete results carry locators.
		return nil
	}
	switch pending.origin {
	case ViewInboxProcessor:
		if m.inboxProcessor != nil {
			return m.inboxProcessor.applyCommittedDelete(pending.identity)
		}
	case ViewGlobalInboxProcessor:
		if m.globalInboxProcessor != nil {
			return m.globalInboxProcessor.applyCommittedDelete(pending.identity)
		}
	default:
		// Only inbox processors arm pendingInboxDeletes.
	}
	return nil
}

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

// clearPendingDeleteBookkeeping drops the Inbox and Weekly Review post-commit
// bookkeeping armed by the delete request identified by token, on that request's
// rejection. It clears only the entries keyed by that token, so an unrelated
// transaction error (token 0, or a different delete's token) never disarms
// another in-flight Inbox or Weekly Review delete that is still awaiting its own
// outcome. It also drops the originating inbox processor's per-task focus slot so
// the in-flight count that gates auto-close (shouldCloseAfterDelete) no longer
// counts the failed delete; the task stays in the queue, so the processor stays
// open and a retry can close it normally.
//
// The processor keys that focus slot by task identity (inboxCore.pendingDeletes), while
// the shell keys this bookkeeping by a unique request token. Because a still-visible
// Inbox task can be confirmed for deletion twice before the first result returns,
// two distinct tokens for the same qualified task can alias ONE processor slot
// (FR-016). Dropping the slot
// unconditionally would let a rejected duplicate delivered first delete the slot the
// matching success still needs to rebuild the queue, mark the task processed, repair
// focus, and auto-close. So the processor slot is dropped only when no OTHER
// in-flight request still targets the same (origin, identity): this token is removed
// first, then the slot survives as long as any sibling request references it.
func (m *AppModel) clearPendingDeleteBookkeeping(token uint64) {
	if token == 0 {
		return
	}
	pending, ok := m.pendingInboxDeletes[token]
	delete(m.pendingInboxDeletes, token)
	if ok && !m.inboxTaskHasOtherPendingDelete(pending.origin, pending.identity) {
		m.clearInboxProcessorPendingDelete(pending.origin, pending.identity)
	}
	delete(m.pendingWeeklyReviewDeletes, token)
}

// inboxTaskHasOtherPendingDelete reports whether any still-armed pendingInboxDeletes
// entry targets the same processor origin and qualified identity. Two request
// tokens for one task share a single slot; a caller must therefore consult this
// before dropping that slot on its own
// request's cancellation or rejection, keeping the shared slot alive until the LAST
// referencing request resolves. Callers remove their own token from
// pendingInboxDeletes first (rejection) or hold no token yet (cancellation, before
// the token is minted), so this counts only the remaining sibling references.
func (m *AppModel) inboxTaskHasOtherPendingDelete(origin PresentationID, identity TaskIdentity) bool {
	if !identity.Valid() {
		return false
	}
	for _, pending := range m.pendingInboxDeletes {
		if pending.origin == origin && pending.identity == identity {
			return true
		}
	}
	return false
}

// clearInboxProcessorPendingDelete drops the per-task focus slot the originating
// inbox processor armed in requestDelete, so a rejected delete no longer counts
// toward the processor's remaining in-flight bookkeeping.
func (m *AppModel) clearInboxProcessorPendingDelete(origin PresentationID, identity TaskIdentity) {
	if !identity.Valid() {
		return
	}
	switch origin {
	case ViewInboxProcessor:
		if m.inboxProcessor != nil {
			delete(m.inboxProcessor.pendingDeletes, identity)
		}
	case ViewGlobalInboxProcessor:
		if m.globalInboxProcessor != nil {
			delete(m.globalInboxProcessor.pendingDeletes, identity)
		}
	default:
		// Only inbox processors own per-task pending-delete slots.
	}
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
		m.cancelInboxPendingDelete()
		m.clearPendingTaskDelete()
	}
	return true, nil
}

// cancelInboxPendingDelete clears the originating inbox processor's per-task
// focus slot armed by requestDelete when a shell-owned Inbox delete confirmation
// is canceled before any correlation token is minted (the token is minted only in
// confirmTaskDelete, so cancellation cannot route through the token-keyed
// clearPendingDeleteBookkeeping). It clears only the canceled request's own task
// entries, so an overlapping Inbox delete on a different task stays armed, and it
// touches no task data or cursor state — only the stale in-flight bookkeeping that
// would otherwise make shouldCloseAfterDelete treat the canceled request as a live
// delete and suppress a later final-task close. A no-op for non-inbox origins.
//
// The processor slot is keyed by task identity, so a canceled duplicate of a task that a
// confirmed, still-in-flight delete already targets must NOT drop that shared slot
// (FR-016) — the in-flight delete still needs it. inboxTaskHasOtherPendingDelete
// sees only confirmed (tokened) requests, so it keeps the slot alive for the sibling
// while still clearing it for an ordinary lone cancellation.
func (m *AppModel) cancelInboxPendingDelete() {
	if !isInboxOrigin(m.pendingTaskDelete.origin) {
		return
	}
	for _, ref := range m.pendingTaskDelete.roots {
		identity := ref.Identity()
		if !identity.Valid() {
			continue
		}
		if m.inboxTaskHasOtherPendingDelete(m.pendingTaskDelete.origin, identity) {
			continue
		}
		m.clearInboxProcessorPendingDelete(m.pendingTaskDelete.origin, identity)
	}
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

	// Inbox processors are not TaskWorkspace-backed: arm post-commit bookkeeping
	// so the processor advances/marks-processed only after this transaction
	// commits. A single inbox root is the only shape inbox surfaces request. Key it
	// by token so a second same-surface delete armed before this one commits gets
	// its own entry instead of overwriting this one.
	if isInboxOrigin(origin) {
		if m.pendingInboxDeletes == nil {
			m.pendingInboxDeletes = make(map[uint64]pendingInboxDelete)
		}
		m.pendingInboxDeletes[token] = pendingInboxDelete{token: token, origin: origin, identity: roots[0].Identity()}
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
				// the shell token slot armed above and the originating inbox processor's
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
