package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
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
	cmd := m.handleWithRefresh(false, false, undoMsg)
	return true, cmd
}

// handleRedoPerformed handles the RedoPerformedMsg.
func (m *AppModel) handleRedoPerformed(msg RedoPerformedMsg) (bool, tea.Cmd) {
	redoMsg := fmt.Sprintf("Redo: %s", msg.Action)
	cmd := m.handleWithRefresh(false, false, redoMsg)
	return true, cmd
}
