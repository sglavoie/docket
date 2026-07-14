package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// crossTogglePinned toggles the pinned state for a task in cross-project mode
// through the single-task coordinator (app SetPinned). The current pinned state
// is read off the row's live task pointer to pick the target value; the
// coordinator commits one ledger-backed transaction and the shell owns reload.
func (m *OutlinerModel) crossTogglePinned(row outlinerRow, _ *models.Project) (tea.Model, tea.Cmd) {
	return m, m.taskItem.SetPinned(row.task.ID, !row.task.Pinned)
}

// crossToggleDimmed toggles the dimmed state for a task in cross-project mode
// through the single-task coordinator (app ToggleDimmed).
func (m *OutlinerModel) crossToggleDimmed(row outlinerRow, _ *models.Project) (tea.Model, tea.Cmd) {
	return m, m.taskItem.ToggleDimmed(row.task.ID)
}

// crossSendTaskToSection moves the cursor task to the specified section in
// cross-project mode through the retired SetSection seam (app layer). This is a
// thin ledger-committed section write; the overlay composition (Activate-if-empty
// and RelocateTaskToEndOfSection) is retired-section design owned by session
// 33.7.3.4.6 and intentionally NOT reintroduced here.
func (m *OutlinerModel) crossSendTaskToSection(section string) (tea.Model, tea.Cmd) {
	row, _, valid := m.crossGetMutationTarget()
	if !valid {
		return m, nil
	}
	return m, m.taskItem.SetSection(row.task.ID, section)
}

// crossHandleTypeChange handles <, >, H, L type change keys in cross-project mode
// through the single-task coordinator (app ChangeTaskType). Multi-select is not
// supported (tasks span projects); shows a status message instead.
func (m *OutlinerModel) crossHandleTypeChange(key string) (tea.Cmd, bool) {
	op, _, ok := typeChangeOpForKey(key)
	if !ok {
		return nil, false
	}
	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		m.setStatus("bulk type change not supported in cross-project mode")
		return nil, true
	}
	row, _, valid := m.crossGetMutationTarget()
	if !valid {
		return nil, true
	}
	return m.taskItem.ChangeType(row.task.ID, appTaskTypeChangeForOp(op)), true
}

// appTaskTypeChangeForOp maps the TUI TypeChangeOp to the application-layer
// TaskTypeChange consumed by Service.ChangeTaskType.
func appTaskTypeChangeForOp(op TypeChangeOp) app.TaskTypeChange {
	switch op {
	case TypePromote:
		return app.PromoteType
	case TypeDemote:
		return app.DemoteType
	case TypeCycleLeft:
		return app.CycleTypeLeft
	case TypeCycleRight:
		return app.CycleTypeRight
	default:
		return app.CycleTypeRight
	}
}

// handleCrossProjectReparent applies a ParentSelectedMsg in cross-project mode
// through the single-task coordinator (app Reparent). The application layer owns
// the parent-existence, type-compatibility, and cycle guards and aborts an
// invalid reparent before any commit, so no undo snapshot is recorded for a
// rejected change. A nil ParentID promotes the task to a root.
func (m *OutlinerModel) handleCrossProjectReparent(msg ParentSelectedMsg) (tea.Model, tea.Cmd) {
	return m, m.taskItem.Reparent(msg.ReparentTaskID, msg.ParentID)
}

// crossExecuteDelete performs a delete of m.deleteTarget in cross-project mode
// through the application-layer BulkCoordinator (single-element). The durable
// planning refs (Focus Queue, Daily Plan, dismissals) of the removed subtree ride
// the delete transaction as a config artifact so the config-ref cleanup is atomic
// with the delete and undoable through the ledger; machine-local recent visits are
// repaired by the shell after the commit from the batch's RemovedTaskIDs.
//
// Called from handleConfirmDialog when m.project==nil and the confirmed action
// is "delete" or "delete-recurring".
func (m *OutlinerModel) crossExecuteDelete() (tea.Model, tea.Cmd) {
	if m.deleteTarget == nil {
		return m, nil
	}
	task := m.deleteTarget
	m.deleteTarget = nil
	m.clearStatus()

	proj := m.crossFindProjectForTask(task.ID)
	if proj == nil {
		m.setStatus("Project for task not found")
		return m, nil
	}

	// Fold the durable planning-ref cleanup for the whole removed subtree into the
	// delete transaction as a config artifact.
	deleteIDs := collectDescendantIDs(proj.Tasks, map[string]bool{task.ID: true})
	var artifacts []storage.ArtifactChange
	if m.runtime != nil && m.configManager != nil {
		planning := clonePlanningState(m.runtime.Planning())
		if planning.RemoveDurableTaskRefs(deleteIDs) > 0 {
			arts, err := planningDomainArtifact(m.configManager, planning)
			if err != nil {
				m.setStatus(fmt.Sprintf("Error: %v", err))
				return m, nil
			}
			artifacts = arts
		}
	}

	return m, m.bulk.Delete([]string{task.ID}, artifacts...)
}
