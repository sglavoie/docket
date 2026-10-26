package tui

import (
	"errors"
	"fmt"

	"github.com/sglavoie/docket/internal/logging"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// workspaceLedgerRestoreIsNewest reports whether the durable workspace ledger
// holds a reversible transaction for the applicable action.
func (m *AppModel) workspaceLedgerRestoreIsNewest(redo bool) bool {
	ledger := workspaceLedgerFromConfig(m.domain.runtime)
	_, ok, err := ledgerNewestEntry(ledger, storage.GlobalScope(), redo)
	return err == nil && ok
}

// ledgerNewestEntry previews the entry a subsequent undo (redo=false) or redo
// (redo=true) would restore for the scope, without applying it.
func ledgerNewestEntry(ledger *storage.WorkspaceLedger, scope storage.LedgerScope, redo bool) (storage.LedgerEntry, bool, error) {
	if redo {
		return ledger.SelectNewestRedo(scope)
	}
	return ledger.SelectNewestUndo(scope)
}

// workspaceHistoryPreview reads the same global entries the undo keys restore.
// Missing or unreadable entries offer no menu action; the direct keys still
// report the ledger's own refusal when invoked.
func (m *AppModel) workspaceHistoryPreview() (undo, redo string) {
	ledger := workspaceLedgerFromConfig(m.domain.runtime)
	if entry, ok, err := ledgerNewestEntry(ledger, storage.GlobalScope(), false); err == nil && ok {
		undo = entry.Description()
	}
	if entry, ok, err := ledgerNewestEntry(ledger, storage.GlobalScope(), true); err == nil && ok {
		redo = entry.Description()
	}
	return undo, redo
}

// handleLedgerTaskUndoRedo restores the newest application-service workspace-ledger
// transaction (undo when redo=false, redo when true). It reverses (or replays) the
// entry, reloads every project the transaction touched, refreshes the runtime
// snapshot so restored planning and other config artifacts are observed, repairs
// task→project references, rebuilds the in-memory planning collections, and fans
// the change out to the open task views — the same refresh the legacy restore
// completion performs. Because it reads the durable ledger rather than in-memory
// state, the restore survives a restart.
//
// The ledger is one workspace history, so an entry can carry catalog effects a
// task view never shows: a tag assignment, an archive, a rename. The
// catalog-reading surfaces are therefore rebuilt from the restored snapshot on
// the same terms every forward catalog mutation rebuilds them.
func (m *AppModel) handleLedgerTaskUndoRedo(redo bool) tea.Cmd {
	cmd, _, _ := m.restoreLedgerEntry(redo, "")
	return cmd
}

// restoreLedgerEntry is handleLedgerTaskUndoRedo's restore, also reporting the
// outcome it puts on the status line and whether the entry was restored, so a
// control request can answer with Docket's own words. A non-empty transactionID
// names the undo entry the caller means: the ledger compares it with the newest
// entry under the restore's own lock and refuses with
// storage.ErrUndoSuperseded, changing nothing, when another commit got there
// first.
func (m *AppModel) restoreLedgerEntry(redo bool, transactionID string) (tea.Cmd, string, bool) {
	m.retireInteraction()
	ledger := workspaceLedgerFromConfig(m.domain.runtime)
	restore := ledger.UndoWithEntry
	verb, label := "Undid", "undo"
	switch {
	case redo:
		restore = ledger.RedoWithEntry
		verb, label = "Redo", "redo"
	case transactionID != "":
		restore = func(scope storage.LedgerScope) (storage.LedgerEntry, string, error) {
			return ledger.UndoTransactionWithEntry(scope, transactionID)
		}
	}
	entry, desc, err := restore(storage.GlobalScope())
	if errors.Is(err, storage.ErrUndoSuperseded) {
		return nil, controlUndoSuperseded, false
	}
	if err != nil {
		// An exhausted ledger is not an error: say so quietly rather than toast
		// a spurious failure.
		if status.IsExhaustedHistory(err) {
			return status.NothingToRestoreCmd(redo), "Nothing to " + label, false
		}
		// A retired entry changed nothing durable but did leave the ledger, so
		// the session memories keyed to it are put down with it.
		var retired *storage.RetiredEntryError
		if errors.As(err, &retired) {
			m.undoFocus.ForgetRetired(retired.Entry.TransactionID, redo)
			m.dueClocks.ForgetRetired(retired.Entry.TransactionID, redo)
		}
		failure := status.FormatUndoRedoRestoreError(label, err)
		return status.ErrorCmd("%s", failure), failure.Error(), false
	}
	restored := make([]*models.Project, 0, len(entry.ProjectIDs))
	for _, id := range entry.ProjectIDs {
		m.reloadProjectByID(id)
		if p := m.projectByID(id); p != nil {
			restored = append(restored, p)
		}
	}
	// A failed runtime reload leaves the pre-restore snapshot in memory: the
	// entry is restored on disk but the views are drawn from before it, so the
	// outcome says so rather than reading as a clean restore.
	reloadErr := m.reloadRuntimeAfterRestore()
	if err := m.repairTaskProjectRefsAfterRestore(restored); err != nil {
		return status.ErrorCmd("%s", err), err.Error(), false
	}
	m.rebuildPlanningFromRuntime()
	m.refreshLoadedTaskWorkspace()
	// The entry's transaction ID is read only here, after the restore actually
	// succeeded: a refused or failed restore changed no durable state and must
	// leave the remembered clocks — and the correlated focus target below —
	// exactly as it found them.
	m.observeRestoredDueClocks(entry.TransactionID, redo)
	// The cursor moves before the views fan out, because the disk-backed reload
	// refreshTaskViewsAfterSave starts captures the cursor's own task: seating it
	// first is what makes the reload land back on the restored row by identity
	// rather than drag the view to wherever it started.
	focusCmd, detail := m.refocusRestoredTask(entry.TransactionID, redo)
	m.clearToggleHistory()
	outcome := appendRestoreDetail(fmt.Sprintf("%s: %s", verb, desc), detail)
	if reloadErr != nil {
		outcome = appendRestoreDetail(outcome, restoreReloadFailedDetail)
		m.setWarningStatus(outcome)
	} else {
		m.setSuccessStatus(outcome)
	}
	m.refreshProjectHubIfActive()
	cmds := m.refreshTaskViewsAfterRestore(reloadErr == nil)
	cmds = append(cmds, focusCmd, m.refreshActiveNotesManager(), m.statusReadingTimeout())
	return tea.Batch(cmds...), outcome, true
}

// restoreReloadFailedDetail ends a restore whose workspace reload failed.
const restoreReloadFailedDetail = "workspace reload failed; restart Docket to see every change"

// reloadRuntimeAfterRestore is refreshRuntime for a ledger restore, which has
// to know why a reload failed rather than only that it did. A shell with no
// workspace on disk has nothing to reload, which is not a failure.
func (m *AppModel) reloadRuntimeAfterRestore() error {
	if m.domain.runtime == nil || m.domain.runtime.DataDir() == "" {
		return nil
	}
	if err := m.domain.runtime.RefreshAfterCommit(); err != nil {
		logging.Warn("failed to refresh runtime domain snapshot after a restore: %v", err)
		return err
	}
	m.syncProjectCache()
	return nil
}

// refocusRestoredTask spends this restore's correlation, if there is one, and
// reports the command the cursor move produced together with the detail the
// status line owes.
//
// The three outcomes are distinct and each says something different. An
// uncorrelated entry — a bulk change, a project mutation, a transaction from
// before this session started — moves nothing and says nothing extra, which is
// the ordinary ledger undo unchanged. A correlated task the projection already
// draws is seated, and only a seating that actually vacated a row advertises the
// return. A correlated task the view does not admit leaves the view exactly as
// it was and names where the restored work now lives, so the answer is on screen
// instead of the user having to go looking for it.
func (m *AppModel) refocusRestoredTask(transactionID string, redo bool) (tea.Cmd, string) {
	address, ok := m.undoFocus.ObserveRestore(transactionID, redo)
	if !ok {
		return nil, ""
	}
	identity := taskref.FromLocator(address)
	task, project := m.taskAndProjectForIdentity(identity)
	if task == nil || project == nil {
		// The restore removed the task at that address — undoing a creation, for
		// instance — so there is nothing to point at and nothing to name.
		return nil, ""
	}
	out := m.crossOutliner
	if out == nil {
		return nil, m.restoredTaskLocation(task, project)
	}
	cmd, seated, originRecorded := out.RefocusRestoredTask(identity)
	if !seated {
		return nil, m.restoredTaskLocation(task, project)
	}
	if !originRecorded {
		return cmd, ""
	}
	return cmd, "Ctrl+O origin"
}

// restoredTaskLocation names where a restored task now lives, in the same terms
// a creation reports its own destination: the owning project and the lane the
// task landed in, planning membership included.
func (m *AppModel) restoredTaskLocation(task *models.Task, project *models.Project) string {
	label := m.createdTaskSectionLabel(task, project)
	if label == "" {
		return fmt.Sprintf("now in %s", project.Name)
	}
	return fmt.Sprintf("now in %s (%s)", project.Name, label)
}

// appendRestoreDetail folds the restore's own detail onto the status line in the
// canonical trailing position. An empty detail leaves the line exactly as the
// ledger worded it.
func appendRestoreDetail(message, detail string) string {
	if detail == "" {
		return message
	}
	return message + " · " + detail
}

func (m *AppModel) repairTaskProjectRefsAfterRestore(projects []*models.Project) error {
	runtimeErr := repairRuntimeTaskRefsAfterRestore(m.domain.runtime)(projects)
	m.rewireTaskProjectRefsAfterRestore(projects)
	return runtimeErr
}

func (m *AppModel) rewireTaskProjectRefsAfterRestore(projects []*models.Project) {
	refs := make(map[taskref.TaskIdentity]TaskProjectRef)
	refsByTaskID := make(map[string][]TaskProjectRef)
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task != nil && task.ID != "" {
				ref := TaskProjectRef{Task: task, Project: project}
				refs[taskref.For(task, project)] = ref
				refsByTaskID[task.ID] = append(refsByTaskID[task.ID], ref)
			}
		}
	}
	resolve := func(task *models.Task, project *models.Project) (TaskProjectRef, bool) {
		if task == nil {
			return TaskProjectRef{}, false
		}
		if project != nil {
			ref, ok := refs[taskref.For(task, project)]
			return ref, ok
		}
		candidates := refsByTaskID[task.ID]
		if len(candidates) == 1 {
			return candidates[0], true
		}
		return TaskProjectRef{}, false
	}

	for i := range m.nextOrder {
		if ref, ok := resolve(m.nextOrder[i].Task, m.nextOrder[i].Project); ok {
			m.nextOrder[i].Task = ref.Task
			m.nextOrder[i].Project = ref.Project
		}
	}
	for i := range m.today {
		if ref, ok := resolve(m.today[i].Task, m.today[i].Project); ok {
			m.today[i].Task = ref.Task
			m.today[i].Project = ref.Project
		}
	}
}
