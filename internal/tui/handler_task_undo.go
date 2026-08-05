package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// handleTaskUndoMessages handles task undo/redo messages.
// Returns true if the message was handled.
func (m *AppModel) handleTaskUndoMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case UndoPerformedMsg:
		return m.handleUndoPerformed(msg)

	case RedoPerformedMsg:
		return m.handleRedoPerformed(msg)
	}
	return false, nil
}

// handleUndoPerformed handles the UndoPerformedMsg.
func (m *AppModel) handleUndoPerformed(msg UndoPerformedMsg) (bool, tea.Cmd) {
	undoMsg := fmt.Sprintf("Undid: %s", msg.Action)
	return m.handleUndoRedoPerformed(undoMsg)
}

// handleRedoPerformed handles the RedoPerformedMsg.
func (m *AppModel) handleRedoPerformed(msg RedoPerformedMsg) (bool, tea.Cmd) {
	redoMsg := fmt.Sprintf("Redo: %s", msg.Action)
	return m.handleUndoRedoPerformed(redoMsg)
}

func (m *AppModel) handleUndoRedoPerformed(statusMsg string) (bool, tea.Cmd) {
	m.clearToggleHistory()
	m.setSuccessStatus(statusMsg)
	m.refreshLoadedTaskWorkspace()
	cmds := m.refreshTaskViewsAfterSave()
	cmds = append(cmds, defaultStatusTimeout())
	return true, tea.Batch(cmds...)
}

// workspaceLedgerRestoreIsNewest reports whether the durable workspace ledger
// holds a reversible transaction for the applicable action.
func (m *AppModel) workspaceLedgerRestoreIsNewest(redo bool) bool {
	ledger := workspaceLedgerFromConfig(m.runtime)
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
	ledger := workspaceLedgerFromConfig(m.runtime)
	restore := ledger.UndoWithEntry
	verb, label := "Undid", "undo"
	if redo {
		restore = ledger.RedoWithEntry
		verb, label = "Redo", "redo"
	}
	entry, desc, err := restore(storage.GlobalScope())
	if err != nil {
		// An exhausted ledger is not an error: leave the key unhandled so the
		// caller can fall back rather than toast a spurious failure.
		if strings.Contains(err.Error(), "history available") {
			return nil
		}
		return errorCmd("%s", formatUndoRedoRestoreError(label, err))
	}
	restored := make([]*models.Project, 0, len(entry.ProjectIDs))
	for _, id := range entry.ProjectIDs {
		m.reloadProjectByID(id)
		if p := m.projectByID(id); p != nil {
			restored = append(restored, p)
		}
	}
	m.refreshRuntime()
	if err := m.repairTaskProjectRefsAfterRestore(restored); err != nil {
		return errorCmd("%s", err)
	}
	m.rebuildPlanningFromRuntime()
	m.refreshLoadedTaskWorkspace()
	m.clearToggleHistory()
	m.setSuccessStatus(fmt.Sprintf("%s: %s", verb, desc))
	m.refreshProjectHubIfActive()
	cmds := m.refreshTaskViewsAfterSave()
	cmds = append(cmds, m.refreshActiveNotesManager(), defaultStatusTimeout())
	return tea.Batch(cmds...)
}

func (m *AppModel) repairTaskProjectRefsAfterRestore(projects []*models.Project) error {
	runtimeErr := repairRuntimeTaskRefsAfterRestore(m.runtime)(projects)
	m.rewireTaskProjectRefsAfterRestore(projects)
	return runtimeErr
}

func (m *AppModel) rewireTaskProjectRefsAfterRestore(projects []*models.Project) {
	refs := make(map[TaskIdentity]TaskProjectRef)
	refsByTaskID := make(map[string][]TaskProjectRef)
	for _, project := range projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task != nil && task.ID != "" {
				ref := TaskProjectRef{Task: task, Project: project}
				refs[taskIdentityFor(task, project)] = ref
				refsByTaskID[task.ID] = append(refsByTaskID[task.ID], ref)
			}
		}
	}
	resolve := func(task *models.Task, project *models.Project) (TaskProjectRef, bool) {
		if task == nil {
			return TaskProjectRef{}, false
		}
		if project != nil {
			ref, ok := refs[taskIdentityFor(task, project)]
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
