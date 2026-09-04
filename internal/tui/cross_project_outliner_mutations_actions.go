package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/listsurface"
)

// handleCrossProjectReparent applies a ParentSelectedMsg in cross-project mode
// through the single-task coordinator (app Reparent). The application layer owns
// the parent-existence, type-compatibility, and cycle guards and aborts an
// invalid reparent before any commit, so no undo snapshot is recorded for a
// rejected change. A nil ParentID promotes the task to a root.
func (m *OutlinerModel) handleCrossProjectReparent(msg listsurface.ParentSelectedMsg) (tea.Model, tea.Cmd) {
	return m, m.taskItem.Reparent(msg.ReparentTask, msg.ParentID)
}
