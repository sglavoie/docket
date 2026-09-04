package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/projecthub"
)

// handleConfirmDialog processes key input while a confirmation dialog is active.
//
// Task deletion is shell-owned, so this handler only owns the Outliner-local
// project archive confirmation.
func (m *OutlinerModel) handleConfirmDialog(key string) (tea.Model, tea.Cmd) {
	// Capture action before HandleKey clears it on confirm/cancel.
	action := m.Confirm.Action()

	// Standard y/n handling
	if confirmed, handled := m.HandleConfirmKey(key); handled {
		if confirmed {
			switch action {
			case projecthub.ProjectArchiveConfirmAction:
				return m, func() tea.Msg { return projecthub.ProjectArchiveConfirmedMsg{} }
			}
		}
		return m, nil
	}
	return m, nil
}
