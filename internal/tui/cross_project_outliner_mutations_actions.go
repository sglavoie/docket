package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// crossTogglePinned toggles the pinned state for a task in cross-project mode
// through the single-task coordinator (app SetPinned). The current pinned state
// is read off the row's live task pointer to pick the target value; the
// coordinator commits one ledger-backed transaction and the shell owns reload.
func (m *OutlinerModel) crossTogglePinned(row outlinerRow, project *models.Project) (tea.Model, tea.Cmd) {
	return m, m.taskItem.SetPinned(taskIdentityFor(row.task, project), !row.task.Pinned)
}

// crossToggleDimmed toggles the dimmed state for a task in cross-project mode
// through the single-task coordinator (app ToggleDimmed).
func (m *OutlinerModel) crossToggleDimmed(row outlinerRow, project *models.Project) (tea.Model, tea.Cmd) {
	return m, m.taskItem.ToggleDimmed(taskIdentityFor(row.task, project))
}

// crossSendTaskToSection moves the cursor task to the specified section in
// cross-project mode through the retired SetSection seam (app layer). This is a
// thin ledger-committed section write; the overlay composition (Activate-if-empty
// and RelocateTaskToEndOfSection) is retired-section design owned by session
// 33.7.3.4.6 and intentionally NOT reintroduced here.
func (m *OutlinerModel) crossSendTaskToSection(section string) (tea.Model, tea.Cmd) {
	row, project, valid := m.crossGetMutationTarget()
	if !valid {
		return m, nil
	}
	label := "Move to " + section
	if section == "" {
		label = "Move to Ready"
	}
	return m, m.taskItem.SetSectionWorkflow(taskIdentityFor(row.task, project), section, m.singleWorkflowOpts(row.task, label), nil)
}

// crossHandleTypeChange handles <, >, H, L type change keys in cross-project mode
// through the single-task coordinator (app ChangeTaskType). Multi-select is not
// supported (tasks span projects); shows a status message instead.
func (m *OutlinerModel) crossHandleTypeChange(key string) (tea.Cmd, bool) {
	op, _, ok := typeChangeOpForKey(key)
	if !ok {
		return nil, false
	}
	if m.multiSelectMode && m.SelectedCount() > 0 {
		m.setStatus("bulk type change not supported in cross-project mode")
		return nil, true
	}
	row, project, valid := m.crossGetMutationTarget()
	if !valid {
		return nil, true
	}
	return m.taskItem.ChangeType(taskIdentityFor(row.task, project), appTaskTypeChangeForOp(op)), true
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
	return m, m.taskItem.Reparent(msg.ReparentTask, msg.ParentID)
}
